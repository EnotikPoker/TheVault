import { useEffect, useRef, useState } from 'react';
import * as api from '../../shared/api/blackbox';
import { ApiError } from '../../shared/api/vaults';
import { loadGames, saveGames } from './storage';

export function useBlackBox() {
  const [session, setSession] = useState<api.BlackBoxSession | null>(null);
  const [daily, setDaily] = useState<api.DailyInfo | null>(null);
  const [busy, setBusy] = useState(true);
  const [needsSync, setNeedsSync] = useState(false);
  const [message, setMessage] = useState('');
  const [receivedAt, setReceivedAt] = useState(0);
  const saved = useRef(loadGames());
  const lock = useRef(false);
  const controller = useRef<AbortController | null>(null);

  function persist() {
    try { saveGames(saved.current); return true; }
    catch { setMessage('Браузер не сохраняет прогресс. Разрешите локальное хранилище для этого сайта.'); return false; }
  }
  function accept(state: api.BlackBoxSession) {
    saved.current.active = state.id;
    if (state.mode === 'daily' && state.date) saved.current.daily[state.date] = state.id;
    else saved.current.practice = state.id;
    if (saved.current.pending?.id === state.id && state.answers.some(a => a.requestId === saved.current.pending?.requestId)) {
      delete saved.current.pending;
    }
    persist();
    setSession(state);
    setReceivedAt(performance.now());
    setNeedsSync(saved.current.pending?.id === state.id);
  }
  async function run(action: (signal: AbortSignal) => Promise<void>, recovering = false) {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setMessage('');
    const abort = controller.current;
    if (!abort) { lock.current = false; setBusy(false); return; }
    try { await action(abort.signal); }
    catch (error) {
      if (abort.signal.aborted) return;
      if (error instanceof ApiError && error.status === 404) {
        setSession(null); setNeedsSync(false);
        if (saved.current.active) {
          const id = saved.current.active;
          delete saved.current.active;
          if (saved.current.practice === id) delete saved.current.practice;
          for (const [day, value] of Object.entries(saved.current.daily)) if (value === id) delete saved.current.daily[day];
          if (saved.current.pending?.id === id) delete saved.current.pending;
          persist();
        }
        setMessage('Сохранённая сессия не найдена. Можно начать новую.');
      } else {
        setNeedsSync(recovering || Boolean(saved.current.active));
        setMessage('Не удалось получить ответ сервера. Восстановите состояние, прежде чем продолжить.');
      }
    } finally { if (controller.current === abort) { lock.current = false; if (!abort.signal.aborted) setBusy(false); } }
  }

  useEffect(() => {
    const abort = new AbortController();
    controller.current = abort;
    void run(async signal => {
      const info = await api.getDaily(signal);
      if (signal.aborted) return;
      setDaily(info);
      if (saved.current.active) {
        const state = await api.getBlackBox(saved.current.active, signal);
        if (!signal.aborted) accept(state);
      }
    });
    const refresh = () => {
      void api.getDaily(abort.signal).then(info => { if (!abort.signal.aborted) setDaily(info); }).catch(() => {});
    };
    const timer = window.setInterval(refresh, 30000);
    document.addEventListener('visibilitychange', refresh);
    return () => { abort.abort(); lock.current = false; window.clearInterval(timer); document.removeEventListener('visibilitychange', refresh); };
  }, []);

  function start(mode: api.Mode, fresh = false) {
    return run(async signal => {
      const info = await api.getDaily(signal); setDaily(info);
      const id = mode === 'daily' ? saved.current.daily[info.date] : fresh ? undefined : saved.current.practice;
      if (fresh && session?.mode === 'practice' && session.status === 'locked'
        && (session.scans.length || session.marks.length || session.answers.length)
        && !window.confirm('Начать новую тренировку? Текущая тренировка будет заменена.')) return;
      accept(id ? await api.getBlackBox(id, signal) : await api.createBlackBox(mode, signal));
    });
  }
  function retry() {
    return run(async signal => {
      setDaily(await api.getDaily(signal));
      const id = saved.current.active;
      if (!id) { setNeedsSync(false); return; }
      const state = await api.getBlackBox(id, signal); accept(state);
      const pending = saved.current.pending;
      if (pending?.id === id) {
        const next = await api.answerBlackBox(id, { requestId: pending.requestId, cells: pending.cells }, signal); accept(next);
        setMessage(next.status === 'opened' ? 'Сейф открыт.' : 'Расположение неверное. Штраф: +5.');
      }
    }, true);
  }
  function scan(port: api.Port) {
    if (!session || needsSync) return;
    return run(async signal => { accept(await api.scanBlackBox(session.id, port, signal)); }, true);
  }
  function mark(cell: api.Cell, state: api.Mark['state']) {
    if (!session || needsSync) return;
    const existing = session.marks.find(m => api.cellKey(m) === api.cellKey(cell));
    const marks = session.marks.filter(m => api.cellKey(m) !== api.cellKey(cell));
    if (existing?.state !== state) marks.push({ ...cell, state });
    if (marks.filter(m => m.state === 'suspected').length > 3) { setMessage('Уже отмечены три элемента. Сначала снимите одну отметку.'); return; }
    return run(async signal => { accept(await api.markBlackBox(session.id, marks, signal)); }, true);
  }
  function answer() {
    if (!session || needsSync) return;
    const cells = session.marks.filter(m => m.state === 'suspected').map(({ row, col }) => ({ row, col }));
    if (cells.length !== 3) return;
    return run(async signal => {
      const bytes = crypto.getRandomValues(new Uint8Array(16));
      const requestId = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
      const pending = { id: session.id, requestId, cells };
      saved.current.pending = pending;
      if (!persist()) { delete saved.current.pending; return; }
      const state = await api.answerBlackBox(session.id, { requestId, cells }, signal);
      accept(state);
      setMessage(state.status === 'opened' ? 'Сейф открыт. Теперь можно изучить пути лучей.' : 'Расположение неверное. Штраф: +5. Отметки можно изменить.');
    }, true);
  }
  return { session, daily, busy, needsSync, message, receivedAt, start, retry, scan, mark, answer };
}
