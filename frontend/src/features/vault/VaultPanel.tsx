import type { Combination, VaultSession } from '../../shared/api/vaults';
import { formatElapsed } from './time';

interface Props {
  session: VaultSession;
  selected: Combination;
  elapsedMs: number;
  disabled: boolean;
  checking: boolean;
  onSelect: (combination: Combination) => void;
  onCheck: () => void;
}

export function VaultPanel({ session, selected, elapsedMs, disabled, checking, onSelect, onCheck }: Props) {
  const last = session.switches;
  const dirty = last !== null && selected.some((enabled, index) => enabled !== last[index]);

  function updateSwitch(index: number, enabled: boolean) {
    const next: Combination = [...selected];
    next[index] = enabled;
    onSelect(next);
  }

  return (
    <section className="vault-panel" aria-label="Панель сейфа">
      <div className="panel-heading">
        <span>CONTROL PANEL</span>
        <span>{session.status === 'opened' ? 'ОТКРЫТ' : 'ЗАКРЫТ'}</span>
      </div>
      <fieldset className="switches" disabled={disabled}>
        <legend>Переключатели</legend>
        {selected.map((enabled, index) => (
          <label className="switch" key={index}>
            <span>S{index + 1}</span>
            <input
              type="checkbox"
              role="switch"
              aria-label={`S${index + 1}`}
              checked={enabled}
              onChange={event => updateSwitch(index, event.currentTarget.checked)}
            />
            <span className="switch-state" aria-hidden="true" />
          </label>
        ))}
      </fieldset>
      <p id="check-note" className="check-note">
        {session.status === 'opened' ? 'Все системы прошли проверку.'
          : last === null ? 'Комбинация ещё не проверена.'
            : dirty ? `Текущая комбинация ещё не проверена. Индикаторы — результат попытки №${session.attempts}.`
              : `Результат последней проверки — попытка №${session.attempts}.`}
      </p>
      <ul className="indicators" aria-label="Индикаторы систем" aria-live="polite">
        {session.indicators.map(({ id, passed }) => (
          <li key={id}>
            <span>{id.toUpperCase()}</span>
            <span className="indicator-result" data-state={passed === null ? 'unknown' : passed ? 'passed' : 'failed'}>
              <i aria-hidden="true" />{passed === null ? 'НЕ ПРОВЕРЕНО' : passed ? 'ПРОЙДЕНО' : 'НЕ ПРОЙДЕНО'}
            </span>
          </li>
        ))}
      </ul>
      <button className="check-button" disabled={disabled} aria-describedby="check-note" onClick={onCheck}>
        {checking ? 'Проверяем…' : 'Проверить комбинацию'}
      </button>
      <div className="metrics">
        <p>Попытки: <strong data-testid="attempt-count">{session.attempts}</strong></p>
        <p>Время: <time aria-label="Прошедшее время">{formatElapsed(elapsedMs)}</time></p>
      </div>
    </section>
  );
}
