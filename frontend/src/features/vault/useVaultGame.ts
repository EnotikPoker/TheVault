import { useEffect, useRef, useState } from 'react';
import { ApiError, checkVault, createVault, getVault } from '../../shared/api/vaults';
import type { Combination, VaultSession } from '../../shared/api/vaults';
import { forgetSession, loadSession, saveSession } from './sessionStorage';

type Operation = 'booting' | 'idle' | 'creating' | 'restoring' | 'checking' | 'recovering';

const emptyCombination = (): Combination => [false, false, false, false, false];

export function useVaultGame() {
  const [session, setSession] = useState<VaultSession | null>(null);
  const [selected, setSelected] = useState<Combination>(emptyCombination);
  const [operation, setOperation] = useState<Operation>('booting');
  const [message, setMessage] = useState<string | null>(null);
  const [needsSync, setNeedsSync] = useState(false);
  const [receivedAt, setReceivedAt] = useState(0);
  const sessionRef = useRef<VaultSession | null>(null);
  const savedId = useRef<string | null>(null);
  const busy = useRef(false);
  const blocked = useRef(false);
  const mounted = useRef(false);
  const generation = useRef(0);
  const controller = useRef<AbortController | null>(null);

  function requireSync(value: boolean) {
    blocked.current = value;
    setNeedsSync(value);
  }

  function apply(state: VaultSession, keepDraft: boolean) {
    sessionRef.current = state;
    savedId.current = state.id;
    setSession(state);
    setReceivedAt(performance.now());
    requireSync(false);
    if (!keepDraft || state.status === 'opened') setSelected(state.switches ?? emptyCombination());
  }

  function expire() {
    forgetSession();
    savedId.current = null;
    sessionRef.current = null;
    setSession(null);
    setSelected(emptyCombination());
    requireSync(false);
    setMessage('Эта игра больше недоступна. Начните новый сейф.');
  }

  function begin(next: Operation) {
    if (busy.current) return null;
    busy.current = true;
    controller.current?.abort();
    const abort = new AbortController();
    controller.current = abort;
    const token = ++generation.current;
    setOperation(next);
    setMessage(null);
    return { signal: abort.signal, current: () => mounted.current && generation.current === token };
  }

  function finish(job: { current: () => boolean }) {
    if (job.current()) {
      busy.current = false;
      setOperation('idle');
    }
  }

  async function restore(id: string, keepDraft: boolean) {
    const job = begin('restoring');
    if (!job) return;
    try {
      const state = await getVault(id, job.signal);
      if (job.current()) apply(state, keepDraft);
    } catch (error) {
      if (!job.current()) return;
      if (error instanceof ApiError && error.status === 404) expire();
      else {
        requireSync(true);
        setMessage('Не удалось загрузить игру. Повторите загрузку состояния.');
      }
    } finally { finish(job); }
  }

  useEffect(() => {
    mounted.current = true;
    busy.current = false;
    const id = loadSession();
    savedId.current = id;
    if (id) void restore(id, false);
    else setOperation('idle');
    return () => {
      mounted.current = false;
      generation.current++;
      controller.current?.abort();
    };
  }, []);

  async function startNew() {
    if (busy.current) return;
    const previous = sessionRef.current;
    if ((previous?.status === 'locked' && previous.attempts > 0) || (blocked.current && savedId.current)) {
      if (!window.confirm('Текущий сейф будет заменён новым. Продолжить?')) return;
    }
    const job = begin('creating');
    if (!job) return;
    try {
      const state = await createVault(job.signal);
      if (job.current()) {
        apply(state, false);
        saveSession(state.id);
      }
    } catch {
      if (job.current()) setMessage('Не удалось создать новый сейф. Попробуйте ещё раз.');
    } finally { finish(job); }
  }

  async function check() {
    const current = sessionRef.current;
    if (!current || current.status === 'opened' || blocked.current) return;
    const job = begin('checking');
    if (!job) return;
    try {
      const state = await checkVault(current.id, selected, job.signal);
      if (job.current()) apply(state, true);
    } catch {
      if (!job.current()) return;
      setOperation('recovering');
      try {
        const state = await getVault(current.id, job.signal);
        if (job.current()) {
          apply(state, true);
          if (state.status !== 'opened') setMessage('Состояние восстановлено. Сверьтесь с журналом перед новой проверкой.');
        }
      } catch (error) {
        if (!job.current()) return;
        if (error instanceof ApiError && error.status === 404) expire();
        else {
          requireSync(true);
          setMessage('Не удалось получить результат проверки. Загрузите состояние игры перед продолжением.');
        }
      }
    } finally { finish(job); }
  }

  function select(combination: Combination) {
    if (!busy.current && !blocked.current && sessionRef.current?.status === 'locked') setSelected([...combination]);
  }

  function retry() {
    if (savedId.current) return restore(savedId.current, sessionRef.current !== null);
  }

  return {
    session, selected, operation, message, needsSync, receivedAt,
    busy: operation !== 'idle',
    startNew, check, select, retry,
  };
}
