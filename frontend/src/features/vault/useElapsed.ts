import { useEffect, useState } from 'react';

export function useElapsed(session: { id: string; status: string; elapsedMs: number } | null, receivedAt: number): number {
  const [now, setNow] = useState(() => performance.now());
  const status = session?.status;
  const id = session?.id;
  const elapsedMs = session?.elapsedMs;

  useEffect(() => {
    setNow(performance.now());
    if (status !== 'locked') return;
    const timer = window.setInterval(() => setNow(performance.now()), 1000);
    return () => window.clearInterval(timer);
  }, [id, status, elapsedMs, receivedAt]);

  if (!session) return 0;
  return session.status === 'opened' ? session.elapsedMs : session.elapsedMs + Math.max(0, now - receivedAt);
}
