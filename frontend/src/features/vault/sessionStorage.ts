import { isSessionId } from '../../shared/api/vaults';

const sessionKey = 'the-vault.session-id';

export function forgetSession(): void {
  try { localStorage.removeItem(sessionKey); } catch { /* Storage can be unavailable. */ }
}

export function loadSession(): string | null {
  try {
    const id = localStorage.getItem(sessionKey);
    if (isSessionId(id)) return id;
    if (id !== null) forgetSession();
  } catch { /* Playing without persistence is still possible. */ }
  return null;
}

export function saveSession(id: string): void {
  try { localStorage.setItem(sessionKey, id); } catch { forgetSession(); }
}
