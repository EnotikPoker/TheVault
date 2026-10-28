import { act, render, renderHook, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { checkVault, createVault, getVault } from '../../shared/api/vaults';
import { checkedSession, draft, initialSession, nextSessionId, openedSession, sessionId, winning } from '../../test/fixtures';
import type { VaultSession } from '../../shared/api/vaults';
import { VaultGame } from './VaultGame';
import { useElapsed } from './useElapsed';
import { formatElapsed } from './time';

vi.mock('../../shared/api/vaults', async (importOriginal) => ({
  ...await importOriginal<typeof import('../../shared/api/vaults')>(),
  createVault: vi.fn(), getVault: vi.fn(), checkVault: vi.fn(),
}));

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(createVault).mockResolvedValue(initialSession());
  vi.mocked(getVault).mockResolvedValue(checkedSession());
  vi.mocked(checkVault).mockResolvedValue(checkedSession());
});

describe('playable panel', () => {
  it('starts by button, submits a draft and restores combinations without a check', async () => {
    const user = userEvent.setup();
    render(<VaultGame />, { reactStrictMode: true });
    expect(createVault).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: 'Начать игру' }));
    const panel = screen.getByRole('region', { name: 'Панель сейфа' });
    const switches = within(panel).getAllByRole('switch');
    expect(switches).toHaveLength(5);
    expect(within(panel).getAllByText('НЕ ПРОВЕРЕНО')).toHaveLength(3);
    expect(screen.getByTestId('attempt-count')).toHaveTextContent('0');
    for (const [index, enabled] of draft.entries()) if (enabled) await user.click(switches[index]!);
    expect(checkVault).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: 'Проверить комбинацию' }));
    await waitFor(() => expect(screen.getByTestId('attempt-count')).toHaveTextContent('1'));
    expect(checkVault).toHaveBeenCalledWith(sessionId, draft, expect.any(AbortSignal));
    expect(within(panel).getByText('НЕ ПРОЙДЕНО')).toBeInTheDocument();
    await user.click(switches[0]!);
    expect(within(panel).getByText(/Текущая комбинация ещё не проверена/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /Вернуть комбинацию из попытки 1/ }));
    expect(switches[0]).toBeChecked();
    expect(within(panel).queryByText(/Текущая комбинация ещё не проверена/)).not.toBeInTheDocument();
    expect(checkVault).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('heading', { name: 'Попытка №1' })).toBeInTheDocument();
  });

  it('locks all game controls during a check without optimistic attempts', async () => {
    let resolve!: (state: VaultSession) => void;
    vi.mocked(checkVault).mockReturnValue(new Promise(done => { resolve = done; }));
    const user = userEvent.setup();
    render(<VaultGame />);
    await user.click(screen.getByRole('button', { name: 'Начать игру' }));
    await user.click(screen.getByRole('button', { name: 'Проверить комбинацию' }));
    expect(screen.getByRole('button', { name: 'Проверяем…' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Новый сейф' })).toBeDisabled();
    for (const input of screen.getAllByRole('switch')) expect(input).toBeDisabled();
    expect(screen.getByTestId('attempt-count')).toHaveTextContent('0');
    await act(async () => { resolve(checkedSession()); });
    expect(screen.getByTestId('attempt-count')).toHaveTextContent('1');
  });

  it('restores an opened game with a fixed winning panel and focused result', async () => {
    localStorage.setItem('the-vault.session-id', sessionId);
    vi.mocked(getVault).mockResolvedValue(openedSession());
    render(<VaultGame />);
    const title = await screen.findByRole('heading', { name: 'Сейф открыт' });
    expect(title).toHaveFocus();
    const switches = screen.getAllByRole('switch');
    for (const [index, enabled] of winning.entries()) {
      expect(switches[index]).toBeDisabled();
      expect((switches[index] as HTMLInputElement).checked).toBe(enabled);
    }
    expect(screen.getByLabelText('Прошедшее время')).toHaveTextContent('00:02');
    expect(screen.getByText('Условие power.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Вернуть комбинацию из попытки 1/ })).toBeDisabled();
    const journal = screen.getByRole('region', { name: 'Журнал проверок' });
    expect(within(journal).getAllByRole('heading', { level: 3 }).map(node => node.textContent)).toEqual(['Попытка №2', 'Попытка №1']);
    expect(createVault).not.toHaveBeenCalled();
  });

  it('starts a new game only after abandonment is confirmed', async () => {
    localStorage.setItem('the-vault.session-id', sessionId);
    vi.mocked(createVault).mockResolvedValue(initialSession(nextSessionId));
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const user = userEvent.setup();
    render(<VaultGame />);
    await screen.findByRole('heading', { name: 'Попытка №1' });
    await user.click(screen.getByRole('button', { name: 'Новый сейф' }));
    expect(createVault).not.toHaveBeenCalled();
    confirm.mockReturnValue(true);
    await user.click(screen.getByRole('button', { name: 'Новый сейф' }));
    await waitFor(() => expect(screen.getByTestId('attempt-count')).toHaveTextContent('0'));
    expect(screen.queryByRole('heading', { name: 'Попытка №1' })).not.toBeInTheDocument();
  });

  it('offers recovery and disables checks after a lost result and failed GET', async () => {
    const user = userEvent.setup();
    vi.mocked(checkVault).mockRejectedValue(new Error('lost response'));
    vi.mocked(getVault).mockRejectedValueOnce(new Error('offline'));
    render(<VaultGame />);
    await user.click(screen.getByRole('button', { name: 'Начать игру' }));
    await user.click(screen.getByRole('button', { name: 'Проверить комбинацию' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Загрузите состояние игры');
    expect(screen.getByRole('button', { name: 'Проверить комбинацию' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: 'Повторить загрузку' }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Проверить комбинацию' })).toBeEnabled());
    expect(screen.getByTestId('attempt-count')).toHaveTextContent('1');
    expect(checkVault).toHaveBeenCalledTimes(1);
  });

  it('allows keyboard play', async () => {
    const user = userEvent.setup();
    render(<VaultGame />);
    await user.tab();
    expect(screen.getByRole('button', { name: 'Начать игру' })).toHaveFocus();
    await user.keyboard('{Enter}');
    await screen.findByRole('switch', { name: 'S1' });
    await user.tab();
    expect(screen.getByRole('switch', { name: 'S1' })).toHaveFocus();
    await user.keyboard(' ');
    expect(screen.getByRole('switch', { name: 'S1' })).toBeChecked();
  });
});

describe('elapsed time', () => {
  it('advances from the server value, resynchronizes, and freezes after opening', () => {
    vi.useFakeTimers();
    const receivedAt = performance.now();
    const { result, rerender } = renderHook(({ state, received }) => useElapsed(state, received), {
      initialProps: { state: checkedSession() as VaultSession, received: receivedAt },
    });
    act(() => { vi.advanceTimersByTime(3000); });
    expect(result.current).toBe(4200);
    rerender({ state: { ...checkedSession(), elapsedMs: 6000 }, received: performance.now() });
    expect(result.current).toBe(6000);
    rerender({ state: openedSession(), received: performance.now() });
    act(() => { vi.advanceTimersByTime(5000); });
    expect(result.current).toBe(2500);
  });

  it.each([[0, '00:00'], [65_999, '01:05'], [3_661_000, '01:01:01']])('formats %i ms as %s', (ms, text) => {
    expect(formatElapsed(ms as number)).toBe(text);
  });
});
