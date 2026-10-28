import type { Attempt, Combination } from '../../shared/api/vaults';
import { formatElapsed } from './time';

interface Props {
  history: Attempt[];
  disabled: boolean;
  onSelect: (combination: Combination) => void;
}

export function VaultHistory({ history, disabled, onSelect }: Props) {
  return (
    <section className="history" aria-labelledby="history-title">
      <h2 id="history-title">Журнал проверок</h2>
      {history.length === 0 ? <p className="muted">Здесь появятся комбинации и результаты ваших экспериментов.</p> : (
        <ol className="history-list">
          {[...history].reverse().map(attempt => (
            <li className="history-entry" key={attempt.attempt}>
              <div className="history-heading"><h3>Попытка №{attempt.attempt}</h3><span>{formatElapsed(attempt.elapsedMs)}</span></div>
              <div className="history-switches">
                {attempt.switches.map((enabled, index) => (
                  <span key={index} className={enabled ? 'enabled' : ''}>S{index + 1} <b>{enabled ? 'ON' : 'OFF'}</b></span>
                ))}
              </div>
              <div className="history-footer">
                <div className="history-indicators">
                  {attempt.indicators.map(({ id, passed }) => (
                    <span key={id} className={passed ? 'passed' : 'failed'} aria-label={`${id.toUpperCase()}: ${passed ? 'пройдено' : 'не пройдено'}`}>
                      {id.toUpperCase()} {passed ? '✓' : '×'}
                    </span>
                  ))}
                </div>
                <button className="secondary-button" disabled={disabled} onClick={() => onSelect(attempt.switches)} aria-label={`Вернуть комбинацию из попытки ${attempt.attempt} на панель`}>На панель</button>
              </div>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
