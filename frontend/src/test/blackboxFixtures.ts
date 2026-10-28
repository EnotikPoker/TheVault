import type { BlackBoxSession, Cell, DailyInfo } from '../shared/api/blackbox';

export const blackboxId = 'c'.repeat(32);
export const practiceId = 'd'.repeat(32);
export const requestId = 'e'.repeat(32);
export const dailyInfo: DailyInfo = { date: '2026-10-28', resetsAt: '2026-10-28T21:00:00Z', size: 6, elementCount: 3 };
export const answerCells: Cell[] = [{ row: 0, col: 0 }, { row: 0, col: 1 }, { row: 0, col: 2 }];
export function blackboxSession(overrides: Partial<BlackBoxSession> = {}): BlackBoxSession {
  return {
    id: blackboxId, mode: 'daily', date: dailyInfo.date, size: 6, elementCount: 3, rulesVersion: 1,
    status: 'locked', createdAt: '2026-10-28T10:00:00Z', elapsedMs: 0, scanCount: 0,
    wrongAnswers: 0, score: 0, marks: [], scans: [], answers: [], ...overrides,
  };
}
export function markedSession(): BlackBoxSession {
  return blackboxSession({ marks: answerCells.map(c => ({ ...c, state: 'suspected' })) });
}
export function scannedSession(): BlackBoxSession {
  return blackboxSession({ elapsedMs: 1500, scanCount: 1, score: 1,
    scans: [{ number: 1, entry: { side: 'top', index: 0 }, result: { kind: 'exit', exit: { side: 'bottom', index: 0 } }, elapsedMs: 1500 }],
  });
}
export function solvedSession(): BlackBoxSession {
  const state = scannedSession();
  const solution = [{ row: 1, col: 3 }, { row: 3, col: 3 }, { row: 4, col: 5 }];
  return { ...state, status: 'opened', elapsedMs: 6000, openedAt: '2026-10-28T10:00:06Z', solution,
    scans: state.scans.map(s => ({ ...s, path: Array.from({ length: 8 }, (_, row) => ({ row: row - 1, col: 0 })) })),
    answers: [{ requestId, cells: solution, correct: true }],
  };
}
