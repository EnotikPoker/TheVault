export type Combination = [boolean, boolean, boolean, boolean, boolean];
export const systemIds = ['power', 'sync', 'access'] as const;
export type SystemID = typeof systemIds[number];

export interface Indicator {
  id: SystemID;
  passed: boolean | null;
}

export interface Attempt {
  attempt: number;
  switches: Combination;
  indicators: Indicator[];
  elapsedMs: number;
}

export interface Explanation {
  id: SystemID;
  condition: string;
  observation: string;
}

interface SessionState {
  id: string;
  createdAt: string;
  attempts: number;
  elapsedMs: number;
  indicators: Indicator[];
  history: Attempt[];
}

export interface LockedSession extends SessionState {
  status: 'locked';
  switches: Combination | null;
}

export interface OpenedSession extends SessionState {
  status: 'opened';
  switches: Combination;
  openedAt: string;
  explanations: Explanation[];
}

export type VaultSession = LockedSession | OpenedSession;

export class ApiError extends Error {
  constructor(public readonly status: number, public readonly code: string) {
    super(`Game API: ${status} ${code}`);
    this.name = 'ApiError';
  }
}

export function isSessionId(value: unknown): value is string {
  return typeof value === 'string' && /^[a-f0-9]{32}$/.test(value);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isCount(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
}

function isDate(value: unknown): value is string {
  return typeof value === 'string' && Number.isFinite(Date.parse(value));
}

function isCombination(value: unknown): value is Combination {
  return Array.isArray(value) && value.length === 5 && value.every(item => typeof item === 'boolean');
}

function isIndicators(value: unknown, neutral = false): value is Indicator[] {
  return Array.isArray(value) && value.length === 3 && value.every((item, index) => (
    isRecord(item) && item.id === systemIds[index]
    && (neutral ? item.passed === null : typeof item.passed === 'boolean')
  ));
}

function isSession(value: unknown): value is VaultSession {
  if (!isRecord(value) || !isSessionId(value.id) || !isDate(value.createdAt)
    || !isCount(value.attempts) || !isCount(value.elapsedMs) || !Array.isArray(value.history)
    || value.history.length !== value.attempts) return false;

  if (!value.history.every((item, index) => isRecord(item) && item.attempt === index + 1
    && isCombination(item.switches) && isIndicators(item.indicators) && isCount(item.elapsedMs))) return false;

  if (value.attempts === 0) {
    if (value.switches !== null || !isIndicators(value.indicators, true)) return false;
  } else if (!isCombination(value.switches) || !isIndicators(value.indicators)) return false;

  if (value.status === 'locked') return value.openedAt === undefined && value.explanations === undefined;
  return value.status === 'opened' && value.attempts > 0 && isCombination(value.switches)
    && isDate(value.openedAt) && isIndicators(value.indicators) && value.indicators.every(item => item.passed)
    && Array.isArray(value.explanations) && value.explanations.length === 3
    && value.explanations.every((item, index) => isRecord(item) && item.id === systemIds[index]
      && typeof item.condition === 'string' && item.condition.length > 0
      && typeof item.observation === 'string' && item.observation.length > 0);
}

async function request(path: string, method: string, body?: unknown, signal?: AbortSignal): Promise<VaultSession> {
  const timeout = AbortSignal.timeout(5000);
  const response = await fetch(path, {
    method,
    ...(body === undefined ? {} : { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
    signal: signal ? AbortSignal.any([signal, timeout]) : timeout,
  });
  const data: unknown = await response.json();
  if (!response.ok) {
    const code = isRecord(data) && isRecord(data.error) && typeof data.error.code === 'string'
      ? data.error.code : 'http_error';
    throw new ApiError(response.status, code);
  }
  if (!isSession(data)) throw new Error('Invalid game state');
  return data;
}

export function createVault(signal?: AbortSignal): Promise<VaultSession> {
  return request('/api/v1/vaults', 'POST', undefined, signal);
}

export async function getVault(id: string, signal?: AbortSignal): Promise<VaultSession> {
  const state = await request(`/api/v1/vaults/${encodeURIComponent(id)}`, 'GET', undefined, signal);
  if (state.id !== id) throw new Error('Unexpected session ID');
  return state;
}

export async function checkVault(id: string, switches: Combination, signal?: AbortSignal): Promise<VaultSession> {
  const state = await request(`/api/v1/vaults/${encodeURIComponent(id)}/checks`, 'POST', { switches }, signal);
  if (state.id !== id) throw new Error('Unexpected session ID');
  return state;
}
