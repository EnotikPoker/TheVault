import { ApiError, isSessionId } from './vaults';

export type Mode = 'daily' | 'practice';
export type Side = 'top' | 'right' | 'bottom' | 'left';
export interface Cell { row: number; col: number }
export interface Port { side: Side; index: number }
export interface Mark extends Cell { state: 'suspected' | 'empty' }
export interface Scan {
  number: number;
  entry: Port;
  result: { kind: 'hit' | 'reflection' | 'exit'; exit?: Port };
  elapsedMs: number;
  path?: Cell[];
}
export interface Answer { requestId: string; cells: Cell[]; correct: boolean }
export interface BlackBoxSession {
  id: string;
  mode: Mode;
  date?: string;
  size: 6;
  elementCount: 3;
  rulesVersion: 1;
  status: 'locked' | 'opened';
  createdAt: string;
  openedAt?: string;
  elapsedMs: number;
  scanCount: number;
  wrongAnswers: number;
  score: number;
  marks: Mark[];
  scans: Scan[];
  answers: Answer[];
  solution?: Cell[];
}
export interface DailyInfo { date: string; resetsAt: string; size: 6; elementCount: 3 }

const record = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v);
const count = (v: unknown): v is number => typeof v === 'number' && Number.isSafeInteger(v) && v >= 0;
const date = (v: unknown): v is string => typeof v === 'string' && Number.isFinite(Date.parse(v));
export const isCell = (v: unknown): v is Cell => record(v) && count(v.row) && v.row < 6 && count(v.col) && v.col < 6;
export const cellKey = (cell: Cell) => `${cell.row}:${cell.col}`;
const cells = (v: unknown): v is Cell[] => Array.isArray(v) && v.length === 3 && v.every(isCell) && new Set(v.map(cellKey)).size === 3;
const port = (v: unknown): v is Port => record(v) && ['top', 'right', 'bottom', 'left'].includes(String(v.side)) && count(v.index) && v.index < 6;

export function isBlackBoxSession(v: unknown): v is BlackBoxSession {
  if (!record(v) || !isSessionId(v.id) || !['daily', 'practice'].includes(String(v.mode))
    || (v.mode === 'daily' && (typeof v.date !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(v.date)))
    || v.size !== 6 || v.elementCount !== 3 || v.rulesVersion !== 1 || !date(v.createdAt)
    || !count(v.elapsedMs) || !count(v.scanCount) || !count(v.wrongAnswers)
    || v.score !== v.scanCount + 5 * v.wrongAnswers || !Array.isArray(v.marks)
    || !v.marks.every(m => isCell(m) && record(m) && ['suspected', 'empty'].includes(String(m.state)))
    || new Set(v.marks.map(m => cellKey(m as Cell))).size !== v.marks.length
    || v.marks.filter(m => (m as Mark).state === 'suspected').length > 3
    || !Array.isArray(v.answers) || !v.answers.every(a => record(a) && isSessionId(a.requestId) && cells(a.cells) && typeof a.correct === 'boolean')
    || v.answers.filter(a => !(a as Answer).correct).length !== v.wrongAnswers
    || !Array.isArray(v.scans) || v.scans.length !== v.scanCount || v.scanCount > 24) return false;
  if (!v.scans.every((s, i) => record(s) && s.number === i + 1 && port(s.entry) && count(s.elapsedMs)
    && record(s.result) && (s.result.kind === 'exit' ? port(s.result.exit)
      : ['hit', 'reflection'].includes(String(s.result.kind)) && s.result.exit === undefined)
    && (v.status === 'locked' ? s.path === undefined : Array.isArray(s.path) && s.path.length > 0
      && s.path.every(p => record(p) && Number.isInteger(p.row) && Number.isInteger(p.col)
        && Number(p.row) >= -1 && Number(p.row) <= 6 && Number(p.col) >= -1 && Number(p.col) <= 6)))) return false;
  if (v.status === 'locked') return v.solution === undefined && v.openedAt === undefined && v.answers.every(a => !(a as Answer).correct);
  return v.status === 'opened' && date(v.openedAt) && cells(v.solution) && v.answers.some(a => (a as Answer).correct);
}

const base = '/api/v1/blackbox';
async function request(path: string, method = 'GET', body?: unknown, signal?: AbortSignal): Promise<unknown> {
  const timeout = AbortSignal.timeout(8000);
  const response = await fetch(base + path, {
    method, signal: signal ? AbortSignal.any([timeout, signal]) : timeout,
    ...(body === undefined ? {} : { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }),
  });
  const data: unknown = await response.json();
  signal?.throwIfAborted();
  if (!response.ok) throw new ApiError(response.status, record(data) && record(data.error) ? String(data.error.code) : 'http_error');
  return data;
}
async function session(path: string, method = 'GET', body?: unknown, signal?: AbortSignal, id?: string) {
  const data = await request(path, method, body, signal);
  if (!isBlackBoxSession(data) || (id !== undefined && data.id !== id)) throw new Error('Invalid scanner session');
  return data;
}
const route = (id: string) => `/sessions/${encodeURIComponent(id)}`;
export async function getDaily(signal?: AbortSignal): Promise<DailyInfo> {
  const data = await request('/daily', 'GET', undefined, signal);
  if (!record(data) || typeof data.date !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(data.date) || !date(data.resetsAt)
    || data.size !== 6 || data.elementCount !== 3) throw new Error('Invalid daily puzzle');
  return data as unknown as DailyInfo;
}
export const createBlackBox = (mode: Mode, signal?: AbortSignal) => session('/sessions', 'POST', { mode }, signal);
export const getBlackBox = (id: string, signal?: AbortSignal) => session(route(id), 'GET', undefined, signal, id);
export const scanBlackBox = (id: string, p: Port, signal?: AbortSignal) => session(`${route(id)}/scans/${p.side}/${p.index}`, 'PUT', undefined, signal, id);
export const markBlackBox = (id: string, marks: Mark[], signal?: AbortSignal) => session(`${route(id)}/marks`, 'PUT', { marks }, signal, id);
export const answerBlackBox = (id: string, answer: Omit<Answer, 'correct'>, signal?: AbortSignal) => session(`${route(id)}/answers`, 'POST', answer, signal, id);
