import { useState } from 'react';

const systems = ['POWER', 'SYNC', 'ACCESS'] as const;

export function VaultPanel() {
  const [switches, setSwitches] = useState([false, false, false, false, false]);

  function updateSwitch(index: number, enabled: boolean) {
    setSwitches(current => current.map((value, position) => position === index ? enabled : value));
  }

  return (
    <section className="vault-panel" aria-label="Панель сейфа">
      <div className="panel-heading">
        <span>CONTROL PANEL</span>
        <span>01 / PROTOTYPE</span>
      </div>
      <fieldset className="switches">
        <legend>Переключатели</legend>
        {switches.map((enabled, index) => (
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
      <ul className="indicators" aria-label="Индикаторы систем">
        {systems.map(name => (
          <li key={name}>
            <span>{name}</span>
            <span className="indicator-result"><i aria-hidden="true" />НЕ ПРОВЕРЕНО</span>
          </li>
        ))}
      </ul>
      <button className="check-button" disabled aria-describedby="prototype-note">
        Проверить комбинацию
      </button>
      <p className="attempts">Попытки: <strong>0</strong></p>
      <p id="prototype-note" className="prototype-note">
        Проверка комбинаций пока недоступна.
      </p>
    </section>
  );
}
