import { describe, expect, it, vi } from 'vitest';
import { ApiError, checkVault, createVault, getVault } from './vaults';
import { checkedSession, draft, initialSession, openedSession, sessionId } from '../../test/fixtures';

describe('game API', () => {
  it('creates, loads and checks using the existing contract', async () => {
    const fetcher = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(initialSession()), { status: 201 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(openedSession())))
      .mockResolvedValueOnce(new Response(JSON.stringify(checkedSession())));
    vi.stubGlobal('fetch', fetcher);
    expect(await createVault()).toEqual(initialSession());
    expect(await getVault(sessionId)).toEqual(openedSession());
    expect(await checkVault(sessionId, draft)).toEqual(checkedSession());
    expect(fetcher.mock.calls[0]?.[1]).toMatchObject({ method: 'POST' });
    expect(fetcher.mock.calls[0]?.[1].body).toBeUndefined();
    expect(fetcher.mock.calls[1]?.[0]).toBe(`/api/v1/vaults/${sessionId}`);
    expect(fetcher.mock.calls[2]?.[1]).toMatchObject({ method: 'POST', body: JSON.stringify({ switches: draft }) });
  });

  it.each([
    null,
    { ...initialSession(), switches: [false] },
    { ...initialSession(), elapsedMs: -1 },
    { ...checkedSession(), history: [] },
    { ...checkedSession(), history: [{ ...checkedSession().history[0], switches: [null, false, false, false, false] }] },
    { ...initialSession(), explanations: [] },
    { ...openedSession(), explanations: [] },
    { ...openedSession(), indicators: initialSession().indicators },
  ])('rejects an invalid session response: %j', async (state) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(state))));
    await expect(createVault()).rejects.toThrow('Invalid game state');
  });

  it('rejects responses for another game', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify(initialSession('b'.repeat(32))))));
    await expect(getVault(sessionId)).rejects.toThrow('Unexpected session ID');
  });

  it('preserves error status and never retries POST', async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: 'already_opened' } }), { status: 409 }));
    vi.stubGlobal('fetch', fetcher);
    await expect(checkVault(sessionId, draft)).rejects.toMatchObject({ status: 409, code: 'already_opened' });
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(new ApiError(404, 'not_found')).toBeInstanceOf(Error);
  });

  it('forwards request cancellation', async () => {
    const controller = new AbortController();
    const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify(initialSession())));
    vi.stubGlobal('fetch', fetcher);
    await createVault(controller.signal);
    const signal = fetcher.mock.calls[0]?.[1].signal as AbortSignal;
    controller.abort();
    expect(signal.aborted).toBe(true);
  });
});
