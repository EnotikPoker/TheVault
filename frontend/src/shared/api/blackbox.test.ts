import { describe, expect, it, vi } from 'vitest';
import { answerCells, blackboxId, blackboxSession, dailyInfo, requestId, scannedSession, solvedSession } from '../../test/blackboxFixtures';
import { answerBlackBox, createBlackBox, getBlackBox, getDaily, isBlackBoxSession, markBlackBox, scanBlackBox } from './blackbox';
import { ApiError } from './vaults';

function respond(value: unknown, status = 200) {
  const fetcher = vi.fn().mockImplementation(async () => new Response(JSON.stringify(value), { status }));
  vi.stubGlobal('fetch', fetcher);
  return fetcher;
}
describe('scanner API', () => {
  it('accepts locked and solved snapshots, without paths in a locked game', () => {
    expect(isBlackBoxSession(blackboxSession())).toBe(true);
    expect(isBlackBoxSession(scannedSession())).toBe(true);
    expect(isBlackBoxSession(solvedSession())).toBe(true);
    expect(isBlackBoxSession({ ...blackboxSession(), solution: answerCells })).toBe(false);
    expect(isBlackBoxSession({ ...scannedSession(), scans: solvedSession().scans })).toBe(false);
    expect(isBlackBoxSession({ ...blackboxSession(), score: 10 })).toBe(false);
    expect(isBlackBoxSession({ ...solvedSession(), solution: [answerCells[0], answerCells[0], answerCells[1]] })).toBe(false);
  });
  it('uses a separate API namespace for creation and daily metadata', async () => {
    const fetcher = respond(blackboxSession(), 201);
    await createBlackBox('daily');
    expect(fetcher).toHaveBeenCalledWith('/api/v1/blackbox/sessions', expect.objectContaining({ method: 'POST', body: '{"mode":"daily"}' }));
    respond(dailyInfo);
    expect(await getDaily()).toEqual(dailyInfo);
  });
  it('uses repeatable port requests and preserves the answer identifier', async () => {
    const fetcher = respond(scannedSession());
    await scanBlackBox(blackboxId, { side: 'top', index: 0 });
    expect(fetcher).toHaveBeenLastCalledWith(`/api/v1/blackbox/sessions/${blackboxId}/scans/top/0`, expect.objectContaining({ method: 'PUT' }));
    await markBlackBox(blackboxId, []);
    expect(fetcher).toHaveBeenLastCalledWith(expect.stringContaining('/marks'), expect.objectContaining({ body: '{"marks":[]}' }));
    await answerBlackBox(blackboxId, { requestId, cells: answerCells });
    expect(fetcher).toHaveBeenLastCalledWith(expect.stringContaining('/answers'), expect.objectContaining({ method: 'POST', body: JSON.stringify({ requestId, cells: answerCells }) }));
  });
  it('rejects mismatched sessions, invalid results and HTTP errors', async () => {
    respond({ ...blackboxSession(), id: 'f'.repeat(32) });
    await expect(getBlackBox(blackboxId)).rejects.toThrow('Invalid');
    respond({ ...scannedSession(), scans: [{ ...scannedSession().scans[0], result: { kind: 'exit' } }] });
    await expect(getBlackBox(blackboxId)).rejects.toThrow('Invalid');
    respond({ error: { code: 'not_found' } }, 404);
    await expect(getBlackBox(blackboxId)).rejects.toEqual(new ApiError(404, 'not_found'));
  });
});
