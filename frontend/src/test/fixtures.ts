import { systemIds } from '../shared/api/vaults';
import type { Attempt, Combination, LockedSession, OpenedSession } from '../shared/api/vaults';

export const sessionId = 'a'.repeat(32);
export const nextSessionId = 'b'.repeat(32);
export const draft: Combination = [true, false, true, false, false];
export const winning: Combination = [false, true, false, true, true];

export function initialSession(id = sessionId): LockedSession {
  return {
    id, createdAt: '2026-10-28T10:00:00Z', status: 'locked', attempts: 0, elapsedMs: 0,
    switches: null, indicators: systemIds.map(id => ({ id, passed: null })), history: [],
  };
}

export function failedAttempt(): Attempt {
  return {
    attempt: 1, switches: [...draft], elapsedMs: 1200,
    indicators: systemIds.map(id => ({ id, passed: id !== 'sync' })),
  };
}

export function checkedSession(): LockedSession {
  const attempt = failedAttempt();
  return { ...initialSession(), attempts: 1, elapsedMs: 1200, switches: attempt.switches, indicators: attempt.indicators, history: [attempt] };
}

export function openedSession(): OpenedSession {
  const indicators = systemIds.map(id => ({ id, passed: true }));
  return {
    ...initialSession(), status: 'opened', attempts: 2, elapsedMs: 2500,
    switches: [...winning], indicators, openedAt: '2026-10-28T10:00:02.500Z',
    history: [failedAttempt(), { attempt: 2, switches: [...winning], indicators, elapsedMs: 2500 }],
    explanations: systemIds.map(id => ({ id, condition: `Условие ${id}.`, observation: `Результат ${id}.` })),
  };
}
