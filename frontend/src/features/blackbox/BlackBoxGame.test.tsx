import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as api from '../../shared/api/blackbox';
import * as legacy from '../../shared/api/vaults';
import { App } from '../../app/App';
import { blackboxId, blackboxSession, dailyInfo, markedSession, scannedSession, solvedSession } from '../../test/blackboxFixtures';
import { initialSession } from '../../test/fixtures';
import { Board } from './Board';
import { BlackBoxGame } from './BlackBoxGame';
import { saveGames } from './storage';

vi.mock('../../shared/api/blackbox', async original => ({
  ...await original<typeof api>(), getDaily: vi.fn(), createBlackBox: vi.fn(), getBlackBox: vi.fn(),
  scanBlackBox: vi.fn(), markBlackBox: vi.fn(), answerBlackBox: vi.fn(),
}));
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.getDaily).mockResolvedValue(dailyInfo);
  vi.mocked(api.createBlackBox).mockResolvedValue(blackboxSession());
  vi.mocked(api.getBlackBox).mockResolvedValue(blackboxSession());
});

describe('scanner interface', () => {
  it('creates on demand, records a ray, and saves free marks separately from guesses', async () => {
    const user = userEvent.setup();
    render(<BlackBoxGame />, { reactStrictMode: true });
    await waitFor(() => expect(screen.getByRole('button', { name: /Сейф дня/ })).toBeEnabled());
    expect(api.createBlackBox).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: /Сейф дня/ }));
    expect(screen.getAllByRole('button', { name: /^Клетка/ })).toHaveLength(36);
    expect(screen.getAllByRole('button', { name: /^Порт/ })).toHaveLength(24);
    vi.mocked(api.scanBlackBox).mockResolvedValue(scannedSession());
    await user.click(screen.getByRole('button', { name: /^Порт N1\./ }));
    expect(screen.getByText('N1 → S1')).toBeInTheDocument();
    expect(screen.queryByRole('img', { name: /^Путь луча/ })).not.toBeInTheDocument();
    vi.mocked(api.markBlackBox).mockImplementation(async (_id, marks) => ({ ...scannedSession(), marks }));
    await user.click(screen.getByRole('button', { name: /^Клетка A1:/ }));
    expect(screen.getByRole('button', { name: /^Клетка A1:/ })).toHaveAttribute('aria-pressed', 'true');
    expect(api.markBlackBox).toHaveBeenCalledWith(blackboxId, [{ row: 0, col: 0, state: 'suspected' }], expect.any(AbortSignal));
    await user.click(screen.getByRole('button', { name: '× Пусто' }));
    await user.click(screen.getByRole('button', { name: /^Клетка B1:/ }));
    expect(api.answerBlackBox).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: /Проверить расположение/ })).toBeDisabled();
  });
  it('locks the board while submitting and shows an incorrect answer without revealing cells', async () => {
    const user = userEvent.setup();
    saveGames({ active: blackboxId, daily: {} });
    vi.mocked(api.getBlackBox).mockResolvedValue(markedSession());
    let resolve!: (state: api.BlackBoxSession) => void;
    vi.mocked(api.answerBlackBox).mockReturnValue(new Promise(done => { resolve = done; }));
    render(<BlackBoxGame />);
    const check = await screen.findByRole('button', { name: /Проверить расположение/ });
    await user.click(check);
    expect(check).toBeDisabled();
    expect(screen.getByRole('button', { name: /^Порт N1\./ })).toBeDisabled();
    const answer = vi.mocked(api.answerBlackBox).mock.calls[0]![1];
    await act(async () => { resolve({ ...markedSession(), score: 5, wrongAnswers: 1, answers: [{ ...answer, correct: false }] }); });
    expect(await screen.findByRole('alert')).toHaveTextContent('Штраф: +5');
    expect(check).toBeEnabled();
    expect(screen.queryByText('Механизм раскрыт')).not.toBeInTheDocument();
  });
  it('shows solution and selected paths after opening, and keeps the final time frozen', async () => {
    const user = userEvent.setup();
    saveGames({ active: blackboxId, daily: {} });
    vi.mocked(api.getBlackBox).mockResolvedValue(solvedSession());
    render(<BlackBoxGame />);
    expect(await screen.findByRole('heading', { name: 'Механизм раскрыт' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Проверить расположение/ })).not.toBeInTheDocument();
    expect(document.querySelectorAll('[data-mark="solution"]')).toHaveLength(3);
    await user.click(screen.getByRole('button', { name: 'Путь луча 1' }));
    expect(screen.getByRole('img', { name: 'Путь луча: N1 → S1' })).toBeInTheDocument();
    expect(document.querySelector('polyline')).toHaveAttribute('points', '1.5,0.5 1.5,1.5 1.5,2.5 1.5,3.5 1.5,4.5 1.5,5.5 1.5,6.5 1.5,7.5');
    expect(document.querySelector('time')).toHaveTextContent('00:06');
    await user.click(screen.getByRole('button', { name: 'Путь луча 1' }));
    expect(screen.queryByRole('img', { name: /^Путь луча/ })).not.toBeInTheDocument();
  });
  it('supports arrow navigation with one board tab stop and explains all ray cases', async () => {
    const user = userEvent.setup();
    const onMark = vi.fn();
    render(<Board session={blackboxSession()} tool="suspected" disabled={false} selectedScan={null} onScan={vi.fn()} onMark={onMark} />);
    const start = screen.getByRole('button', { name: /^Клетка A1:/ });
    act(() => start.focus());
    await user.keyboard('{ArrowRight}{ArrowDown}{Enter}');
    expect(onMark).toHaveBeenCalledWith({ row: 1, col: 1 });
    expect(document.querySelectorAll('.bb-board button[tabindex="0"]')).toHaveLength(1);
  });
  it('defaults to scanner and still allows playing the switch mode', async () => {
    const user = userEvent.setup();
    const create = vi.spyOn(legacy, 'createVault').mockResolvedValue(initialSession());
    render(<App />);
    expect(screen.getByRole('region', { name: 'Сканирование механизма' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Переключатели' }));
    await user.click(screen.getByRole('button', { name: 'Начать игру' }));
    expect(await screen.findByRole('region', { name: 'Панель сейфа' })).toBeInTheDocument();
    expect(create).toHaveBeenCalledOnce();
  });
});
