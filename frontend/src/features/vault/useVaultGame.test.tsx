import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, checkVault, createVault, getVault } from '../../shared/api/vaults';
import type { VaultSession } from '../../shared/api/vaults';
import { checkedSession, draft, initialSession, nextSessionId, openedSession, sessionId, winning } from '../../test/fixtures';
import { useVaultGame } from './useVaultGame';

vi.mock('../../shared/api/vaults', async (importOriginal) => ({
  ...await importOriginal<typeof import('../../shared/api/vaults')>(),
  createVault: vi.fn(), getVault: vi.fn(), checkVault: vi.fn(),
}));

const key = 'the-vault.session-id';
const create = vi.mocked(createVault);
const get = vi.mocked(getVault);
const check = vi.mocked(checkVault);

beforeEach(() => {
  vi.resetAllMocks();
  create.mockResolvedValue(initialSession());
  get.mockResolvedValue(checkedSession());
  check.mockResolvedValue(checkedSession());
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(done => { resolve = done; });
  return { promise, resolve };
}

describe('game session lifecycle', () => {
  it('starts only on demand and ignores double presses in StrictMode', async () => {
    const pending = deferred<VaultSession>();
    create.mockReturnValue(pending.promise);
    const { result } = renderHook(useVaultGame, { reactStrictMode: true });
    expect(create).not.toHaveBeenCalled();
    expect(get).not.toHaveBeenCalled();
    let started: Promise<void>;
    act(() => {
      started = result.current.startNew();
      void result.current.startNew();
    });
    expect(create).toHaveBeenCalledTimes(1);
    expect(result.current.session).toBeNull();
    await act(async () => { pending.resolve(initialSession()); await started; });
    expect(result.current.session).toEqual(initialSession());
    expect(localStorage.getItem(key)).toBe(sessionId);
  });

  it('restores the last checked combination and the server history', async () => {
    localStorage.setItem(key, sessionId);
    const { result } = renderHook(useVaultGame);
    await waitFor(() => expect(result.current.session).toEqual(checkedSession()));
    expect(result.current.selected).toEqual(draft);
    expect(create).not.toHaveBeenCalled();
  });

  it('ignores a corrupt stored ID', () => {
    localStorage.setItem(key, '{invalid}');
    const { result } = renderHook(useVaultGame);
    expect(result.current.operation).toBe('idle');
    expect(localStorage.getItem(key)).toBeNull();
    expect(get).not.toHaveBeenCalled();
  });

  it('clears expired sessions without automatically creating another', async () => {
    localStorage.setItem(key, sessionId);
    get.mockRejectedValue(new ApiError(404, 'not_found'));
    const { result } = renderHook(useVaultGame);
    await waitFor(() => expect(result.current.operation).toBe('idle'));
    expect(result.current.message).toContain('больше недоступна');
    expect(localStorage.getItem(key)).toBeNull();
    expect(create).not.toHaveBeenCalled();
  });

  it('retains the saved ID after temporary failure and retries GET', async () => {
    localStorage.setItem(key, sessionId);
    get.mockRejectedValueOnce(new Error('offline'));
    const { result } = renderHook(useVaultGame);
    await waitFor(() => expect(result.current.needsSync).toBe(true));
    expect(localStorage.getItem(key)).toBe(sessionId);
    await act(async () => { await result.current.retry(); });
    expect(result.current.session).toEqual(checkedSession());
    expect(result.current.needsSync).toBe(false);
  });

  it('recovers a lost check response with GET, without resubmitting POST', async () => {
    check.mockRejectedValue(new Error('response lost'));
    get.mockResolvedValue(openedSession());
    const { result } = renderHook(useVaultGame);
    await act(async () => { await result.current.startNew(); });
    act(() => result.current.select(draft));
    await act(async () => { await result.current.check(); });
    expect(check).toHaveBeenCalledTimes(1);
    expect(get).toHaveBeenCalledTimes(1);
    expect(result.current.session).toEqual(openedSession());
    expect(result.current.selected).toEqual(winning);
  });

  it('blocks further checks until recovery succeeds and keeps the draft', async () => {
    check.mockRejectedValue(new Error('response lost'));
    get.mockRejectedValueOnce(new Error('offline'));
    const { result } = renderHook(useVaultGame);
    await act(async () => { await result.current.startNew(); });
    act(() => result.current.select(winning));
    await act(async () => { await result.current.check(); });
    expect(result.current.needsSync).toBe(true);
    await act(async () => { await result.current.check(); });
    expect(check).toHaveBeenCalledTimes(1);
    await act(async () => { await result.current.retry(); });
    expect(result.current.selected).toEqual(winning);
    expect(result.current.session?.attempts).toBe(1);
    expect(result.current.needsSync).toBe(false);
  });

  it('confirms abandonment and preserves the old game if creation fails', async () => {
    localStorage.setItem(key, sessionId);
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const { result } = renderHook(useVaultGame);
    await waitFor(() => expect(result.current.session?.attempts).toBe(1));
    await act(async () => { await result.current.startNew(); });
    expect(create).not.toHaveBeenCalled();
    confirm.mockReturnValue(true);
    create.mockRejectedValueOnce(new Error('offline'));
    await act(async () => { await result.current.startNew(); });
    expect(result.current.session).toEqual(checkedSession());
    expect(localStorage.getItem(key)).toBe(sessionId);
    create.mockResolvedValueOnce(initialSession(nextSessionId));
    await act(async () => { await result.current.startNew(); });
    expect(result.current.session?.attempts).toBe(0);
    expect(localStorage.getItem(key)).toBe(nextSessionId);
  });

  it('continues playing when localStorage is unavailable', async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked'); });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('blocked'); });
    const { result } = renderHook(useVaultGame);
    await act(async () => { await result.current.startNew(); });
    expect(result.current.session).toEqual(initialSession());
    expect(result.current.busy).toBe(false);
  });

  it('ignores an old restore response and cancels obsolete requests', async () => {
    localStorage.setItem(key, sessionId);
    const old = deferred<VaultSession>();
    get.mockReturnValueOnce(old.promise).mockResolvedValueOnce(checkedSession());
    const { result, unmount } = renderHook(useVaultGame, { reactStrictMode: true });
    await waitFor(() => expect(result.current.session?.attempts).toBe(1));
    expect(get.mock.calls[0]?.[1]?.aborted).toBe(true);
    await act(async () => { old.resolve(initialSession()); });
    expect(result.current.session?.attempts).toBe(1);
    const signal = get.mock.calls[1]?.[1];
    unmount();
    expect(signal?.aborted).toBe(true);
  });
});
