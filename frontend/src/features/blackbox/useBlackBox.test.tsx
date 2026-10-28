import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import * as api from '../../shared/api/blackbox';
import { ApiError } from '../../shared/api/vaults';
import { answerCells, blackboxId, blackboxSession, dailyInfo, markedSession, practiceId, requestId, scannedSession } from '../../test/blackboxFixtures';
import { loadGames, saveGames } from './storage';
import { useBlackBox } from './useBlackBox';

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
async function ready() {
  const hook = renderHook(useBlackBox, { reactStrictMode: true });
  await waitFor(() => expect(hook.result.current.busy).toBe(false));
  return hook;
}
describe('scanner sessions', () => {
  it('creates on demand and prevents two simultaneous starts', async () => {
    const { result } = await ready();
    expect(api.createBlackBox).not.toHaveBeenCalled();
    await act(async () => { await Promise.all([result.current.start('daily'), result.current.start('daily')]); });
    expect(api.createBlackBox).toHaveBeenCalledTimes(1);
    expect(loadGames().daily[dailyInfo.date]).toBe(blackboxId);
  });
  it('restores daily and practice independently, without creating a second daily session', async () => {
    saveGames({ daily: { [dailyInfo.date]: blackboxId }, active: practiceId, practice: practiceId });
    vi.mocked(api.getBlackBox).mockImplementation(async id => blackboxSession({ id, mode: id === practiceId ? 'practice' : 'daily' }));
    const { result } = await ready();
    expect(result.current.session?.id).toBe(practiceId);
    await act(async () => { await result.current.start('daily'); });
    expect(result.current.session?.id).toBe(blackboxId);
    await act(async () => { await result.current.start('practice'); });
    expect(result.current.session?.id).toBe(practiceId);
    expect(api.createBlackBox).not.toHaveBeenCalled();
  });
  it('recovers a lost scan response with GET and blocks further actions until recovery', async () => {
    saveGames({ active: blackboxId, daily: {} });
    vi.mocked(api.scanBlackBox).mockRejectedValue(new Error('lost'));
    const { result } = await ready();
    await act(async () => { await result.current.scan({ side: 'top', index: 0 }); });
    expect(result.current.needsSync).toBe(true);
    await act(async () => { await result.current.scan({ side: 'top', index: 1 }); });
    expect(api.scanBlackBox).toHaveBeenCalledTimes(1);
    vi.mocked(api.getBlackBox).mockResolvedValue(scannedSession());
    await act(async () => { await result.current.retry(); });
    expect(result.current.session?.score).toBe(1);
    expect(result.current.needsSync).toBe(false);
  });
  it('persists an answer before sending and retries exactly the same content after a lost response', async () => {
    saveGames({ active: blackboxId, daily: {} });
    vi.mocked(api.getBlackBox).mockResolvedValue(markedSession());
    vi.mocked(api.answerBlackBox).mockImplementationOnce(async (_id, answer) => {
      expect(loadGames().pending?.requestId).toBe(answer.requestId);
      throw new Error('lost');
    });
    const { result, unmount } = await ready();
    await act(async () => { await result.current.answer(); });
    const pending = loadGames().pending!;
    expect(pending.cells).toEqual(answerCells);
    expect(result.current.needsSync).toBe(true);
    unmount();
    const restored = await ready();
    const answered = blackboxSession({ ...markedSession(), score: 5, wrongAnswers: 1, answers: [{ requestId: pending.requestId, cells: answerCells, correct: false }] });
    vi.mocked(api.answerBlackBox).mockResolvedValue(answered);
    await act(async () => { await restored.result.current.retry(); });
    expect(api.answerBlackBox).toHaveBeenLastCalledWith(blackboxId, { requestId: pending.requestId, cells: answerCells }, expect.any(AbortSignal));
    expect(restored.result.current.session?.score).toBe(5);
    expect(loadGames().pending).toBeUndefined();
  });
  it('recognizes an already recorded pending answer even when it is not the last answer', async () => {
    saveGames({ active: blackboxId, daily: {}, pending: { id: blackboxId, requestId, cells: answerCells } });
    vi.mocked(api.getBlackBox).mockResolvedValue(blackboxSession({ score: 10, wrongAnswers: 2, answers: [
      { requestId, cells: answerCells, correct: false }, { requestId: 'f'.repeat(32), cells: answerCells, correct: false },
    ] }));
    const { result } = await ready();
    expect(result.current.needsSync).toBe(false);
    expect(loadGames().pending).toBeUndefined();
    expect(api.answerBlackBox).not.toHaveBeenCalled();
  });
  it('keeps an unfinished daily session after Moscow midnight and starts the new day separately', async () => {
    saveGames({ active: blackboxId, daily: { [dailyInfo.date]: blackboxId } });
    vi.mocked(api.getDaily).mockResolvedValue({ ...dailyInfo, date: '2026-10-29', resetsAt: '2026-10-29T21:00:00Z' });
    const { result } = await ready();
    expect(result.current.session?.date).toBe('2026-10-28');
    vi.mocked(api.createBlackBox).mockResolvedValue(blackboxSession({ id: practiceId, date: '2026-10-29' }));
    await act(async () => { await result.current.start('daily'); });
    expect(loadGames().daily).toEqual({ '2026-10-28': blackboxId, '2026-10-29': practiceId });
  });
  it('does not submit an answer if its retry identifier cannot be saved', async () => {
    saveGames({ active: blackboxId, daily: {} });
    vi.mocked(api.getBlackBox).mockResolvedValue(markedSession());
    const { result } = await ready();
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('denied'); });
    await act(async () => { await result.current.answer(); });
    expect(api.answerBlackBox).not.toHaveBeenCalled();
    expect(result.current.message).toContain('не сохраняет');
  });
  it('does not allow a fourth element and confirms replacement of an unfinished practice', async () => {
    saveGames({ active: practiceId, daily: {}, practice: practiceId });
    vi.mocked(api.getBlackBox).mockResolvedValue({ ...markedSession(), mode: 'practice', id: practiceId });
    const { result } = await ready();
    await act(async () => { await result.current.mark({ row: 3, col: 3 }, 'suspected'); });
    expect(api.markBlackBox).not.toHaveBeenCalled();
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    await act(async () => { await result.current.start('practice', true); });
    expect(window.confirm).toHaveBeenCalledOnce();
    expect(api.createBlackBox).not.toHaveBeenCalled();
  });
  it('retains session IDs after temporary failure and clears a missing session', async () => {
    saveGames({ active: blackboxId, daily: { [dailyInfo.date]: blackboxId } });
    vi.mocked(api.getBlackBox).mockRejectedValue(new Error('offline'));
    const { result } = await ready();
    expect(result.current.needsSync).toBe(true);
    expect(loadGames().active).toBe(blackboxId);
    vi.mocked(api.getBlackBox).mockRejectedValue(new ApiError(404, 'not_found'));
    await act(async () => { await result.current.retry(); });
    expect(result.current.needsSync).toBe(false);
    expect(loadGames().active).toBeUndefined();
  });
});
