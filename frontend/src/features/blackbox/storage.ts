import { isCell, type Cell } from '../../shared/api/blackbox';
import { isSessionId } from '../../shared/api/vaults';

export const storageKey = 'thevault.blackbox.v1';
export interface PendingAnswer { id: string; requestId: string; cells: Cell[] }
export interface SavedGames { active?: string; daily: Record<string, string>; practice?: string; pending?: PendingAnswer }
export function loadGames(): SavedGames {
  try {
    const v = JSON.parse(localStorage.getItem(storageKey) ?? 'null') as SavedGames | null;
    if (!v || typeof v.daily !== 'object' || v.daily === null || Array.isArray(v.daily)) return { daily: {} };
    const daily = Object.fromEntries(Object.entries(v.daily).filter(([date, id]) => /^\d{4}-\d{2}-\d{2}$/.test(date) && isSessionId(id)));
    const pending = v.pending;
    return {
      daily, ...(isSessionId(v.active) ? { active: v.active } : {}), ...(isSessionId(v.practice) ? { practice: v.practice } : {}),
      ...(pending && isSessionId(pending.id) && isSessionId(pending.requestId) && Array.isArray(pending.cells)
        && pending.cells.length === 3 && pending.cells.every(isCell)
        && new Set(pending.cells.map(c => `${c.row}:${c.col}`)).size === 3 ? { pending } : {}),
    };
  } catch { return { daily: {} }; }
}
export function saveGames(games: SavedGames) { localStorage.setItem(storageKey, JSON.stringify(games)); }
