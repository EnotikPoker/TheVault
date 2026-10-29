import { useState } from 'react';
import type { Combination } from '../../shared/api/vaults';

interface Example {
  title: string;
  action: string;
  switches: Combination;
  passed: [boolean, boolean, boolean];
  observation: string;
}

const examples: Example[] = [
  {
    title: 'Зафиксируем исходную комбинацию', switches: [false, false, false, false, false], passed: [false, false, false],
    action: 'Начнём со всех OFF и проверим. Пока никаких догадок не нужно: нам нужен результат для сравнения.',
    observation: 'Все три системы красные. Это значит только, что эта комбинация не выполняет их условия. Красный не указывает на конкретный неправильный переключатель.',
  },
  {
    title: 'Изменим только S1', switches: [true, false, false, false, false], passed: [true, true, false],
    action: 'Включим S1, оставив остальные OFF. Сравним с попыткой 1.',
    observation: 'POWER и SYNC стали зелёными. S1 влияет на обе системы. ACCESS остался красным: его условие ещё не выполнено, но из этого нельзя заключить, что S1 в нём не участвует.',
  },
  {
    title: 'Проверим влияние S2', switches: [true, true, false, false, false], passed: [false, true, true],
    action: 'Оставим S1 включённым и включим S2. Сравним с попыткой 2: изменился только S2.',
    observation: 'ACCESS стал зелёным, но POWER — красным. Просто включать всё подряд не подходит. Для POWER можно предположить связь между S1 и S2; одно наблюдение ещё не доказывает конкретное правило.',
  },
  {
    title: 'Вернём основу и попробуем S3', switches: [true, false, true, false, false], passed: [true, false, true],
    action: 'Возьмём попытку 2 из журнала кнопкой «На панель»: только S1 включён. Затем включим S3. Сравниваем именно с попыткой 2.',
    observation: 'Теперь ACCESS зелёный, но SYNC стал красным. В этом сочетании S3 не подходит для общего решения. Возврат комбинации из журнала бесплатен; попытка тратится только на новую проверку.',
  },
  {
    title: 'Проверим S5 на той же основе', switches: [true, false, false, false, true], passed: [true, true, true],
    action: 'Снова вернём попытку 2: только S1 включён. Теперь включим S5 вместо S3 и проверим.',
    observation: 'Все три системы зелёные одновременно — сейф открыт. Важна одна общая комбинация: зелёные результаты из разных попыток не складываются.',
  },
];

export function SwitchExample() {
  const [step, setStep] = useState(0);
  const [shown, setShown] = useState(false);
  const example = examples[step]!;
  function go(next: number) { setStep(next); setShown(false); }
  return (
    <section className="guide-example" aria-label="Учебный пример переключателей">
      <p className="guide-example-label">УЧЕБНЫЙ СЕЙФ · БЕЗ РЕАЛЬНЫХ ПОПЫТОК</p>
      <h3>Разбор от первой проверки до открытия</h3>
      <p>Здесь заранее выбран один пример. В вашей игре условия могут быть другими. Эти кнопки не меняют настоящий сейф.</p>
      <p className="guide-progress">Шаг {step + 1} из {examples.length}</p>
      <h4>{example.title}</h4>
      <p>{example.action}</p>
      <ul className="guide-example-switches" aria-label="Учебная комбинация">
        {example.switches.map((on, index) => <li key={index} data-on={on}><span>S{index + 1}</span><b>{on ? 'ON' : 'OFF'}</b></li>)}
      </ul>
      <button onClick={() => setShown(true)}>Проверить учебную комбинацию</button>
      {shown && <div className="guide-example-result" role="status">
        <p><strong>Результат учебной проверки {step + 1}</strong></p>
        <ul className="guide-example-indicators" aria-label="Учебные индикаторы">
          {['POWER', 'SYNC', 'ACCESS'].map((name, index) => <li key={name}><span>{name}</span><span className={example.passed[index] ? 'passed' : 'failed'}>{example.passed[index] ? '✓ ПРОЙДЕНО' : '× НЕ ПРОЙДЕНО'}</span></li>)}
        </ul>
        <p>{example.observation}</p>
      </div>}
      {shown && step === examples.length - 1 && <div className="guide-note">
        <h4>Какие правила были в этом примере</h4>
        <ul>
          <li><strong>POWER:</strong> S1 и S2 в разных положениях.</li>
          <li><strong>SYNC:</strong> ровно один ON среди S1, S3 и S4.</li>
          <li><strong>ACCESS:</strong> ровно один ON среди S2, S3, S4 и S5.</li>
        </ul>
        <p>S1 выполняет первые два условия, а S5 — третье. Остальные OFF. Так выглядит объяснение, которое настоящий сейф показывает после вскрытия.</p>
      </div>}
      <div className="guide-example-actions">
        <button className="secondary-button" disabled={step === 0} onClick={() => go(step - 1)}>Предыдущий шаг</button>
        {step < examples.length - 1 ? <button disabled={!shown} onClick={() => go(step + 1)}>Следующий шаг</button>
          : <button className="secondary-button" onClick={() => go(0)}>Пройти пример заново</button>}
      </div>
    </section>
  );
}
