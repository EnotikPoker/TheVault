import { useId, useState } from 'react';
import type { Cell } from '../../shared/api/blackbox';

const elements: Cell[] = [{ row: 0, col: 2 }, { row: 2, col: 1 }, { row: 2, col: 3 }];
const examples = [
  {
    title: 'Луч идёт прямо', port: 'W5', outcome: 'W5 → E5: выход с другой стороны',
    prompt: 'Порт W5 находится слева от пятой строки. Здесь рядом с лучом нет элементов. Посмотрите, где он выйдет.',
    explanation: 'Луч прошёл по пятой строке и вышел в E5. В игре оба порта получат одинаковый номер сканирования. Это номер записи в журнале, а не количество элементов.',
    path: [{ row: 4, col: -1 }, ...Array.from({ length: 6 }, (_, col) => ({ row: 4, col })), { row: 4, col: 6 }],
  },
  {
    title: 'Элемент поглощает луч', port: 'N3', outcome: 'N3 → поглощение (×)',
    prompt: 'N3 находится сверху, над столбцом C. В первой клетке C1 стоит элемент.',
    explanation: 'Элемент прямо перед лучом поглотил его. В настоящей игре вы увидите × на порту, но не узнаете клетку попадания: до поглощения луч мог поворачивать.',
    path: [{ row: -1, col: 2 }, { row: 0, col: 2 }],
  },
  {
    title: 'Диагональный элемент отклоняет луч', port: 'W2', outcome: 'W2 → N1: поворот на 90°',
    prompt: 'Из W2 луч входит в A2 и движется вправо. Элемент B3 оказывается впереди по диагонали снизу.',
    explanation: 'Ещё в A2 луч повернул от элемента — вверх. Он прошёл через A1 и вышел в N1. Сам элемент B3 луч не задел. После поворота «впереди» определяется уже по новому направлению.',
    path: [{ row: 1, col: -1 }, { row: 1, col: 0 }, { row: 0, col: 0 }, { row: -1, col: 0 }],
  },
  {
    title: 'Два элемента разворачивают луч', port: 'S3', outcome: 'S3 → возврат (↩)',
    prompt: 'Из нижнего порта S3 луч идёт вверх по столбцу C. В C4 впереди по диагоналям оказываются B3 и D3.',
    explanation: 'Луч развернулся ещё в C4 и вернулся через C5 и C6 в исходный S3. На поле линия туда и обратно совпадает. В игре вы увидите ↩, без внутреннего пути.',
    path: [{ row: 6, col: 2 }, { row: 5, col: 2 }, { row: 4, col: 2 }, { row: 3, col: 2 }, { row: 4, col: 2 }, { row: 5, col: 2 }, { row: 6, col: 2 }],
  },
  {
    title: 'Возврат ещё до входа', port: 'N2', outcome: 'N2 → возврат (↩)',
    prompt: 'У входа N2 клетка B1 свободна, но справа от неё стоит элемент C1.',
    explanation: 'Диагональный элемент у самого входа отражает луч до входа на поле. Результат тот же ↩, что и при развороте внутри. По одному символу эти случаи не различить. Если бы B1 тоже была занята, сначала сработало бы поглощение.',
    path: [{ row: -1, col: 1 }],
  },
];

export function ScannerExample() {
  const [step, setStep] = useState(0);
  const [shown, setShown] = useState(false);
  const id = useId();
  const example = examples[step]!;
  function go(next: number) { setStep(next); setShown(false); }
  return (
    <section className="guide-example" aria-label="Учебный макет сканера">
      <p className="guide-example-label">УЧЕБНЫЙ МАКЕТ · БЕЗ ОЧКОВ И ТАЙМЕРА</p>
      <h3>Посмотрите, как движется луч</h3>
      <p>Это отдельный пример: три элемента специально видны в C1, B3 и D3. В вашем сейфе они спрятаны. Нажатия здесь не меняют вашу игру.</p>
      <p className="guide-progress">Пример {step + 1} из {examples.length}</p>
      <h4>{example.title}</h4>
      <p>{example.prompt}</p>
      <svg className="guide-scanner-diagram" viewBox="0 0 8 8" role="img" aria-label={shown ? `Учебный луч: ${example.outcome}` : 'Учебное поле: элементы в C1, B3 и D3'}>
        <defs><marker id={id} markerWidth="5" markerHeight="5" refX="4" refY="2.5" orient="auto"><path d="M0 0L5 2.5L0 5" fill="#92e3f1" /></marker></defs>
        {Array.from({ length: 36 }, (_, n) => {
          const row = Math.floor(n / 6), col = n % 6;
          return <g key={n}><rect x={col + 1} y={row + 1} width="1" height="1" fill="#1b2523" stroke="#59665d" strokeWidth=".02" />
            <text x={col + 1.08} y={row + 1.22} fontSize=".18" fill="#b8c3b9">{String.fromCharCode(65 + col)}{row + 1}</text></g>;
        })}
        {Array.from({ length: 6 }, (_, n) => <g key={n} fill="#c0cdc2" fontSize=".23" textAnchor="middle">
          <text x={n + 1.5} y=".25">N{n + 1}</text><text x={n + 1.5} y="7.95">S{n + 1}</text>
          <text x=".2" y={n + 1.58}>W{n + 1}</text><text x="7.8" y={n + 1.58}>E{n + 1}</text>
        </g>)}
        {elements.map(c => <circle key={`${c.row}:${c.col}`} cx={c.col + 1.5} cy={c.row + 1.5} r=".2" fill="#e4ba77" />)}
        {shown && <g>
          {example.path.length > 1 && <polyline className="guide-ray-line" points={example.path.map(c => `${c.col + 1.5},${c.row + 1.5}`).join(' ')} markerEnd={`url(#${id})`} />}
          <circle cx={example.path[0]!.col + 1.5} cy={example.path[0]!.row + 1.5} r=".11" fill="#92e3f1" />
          {example.path.length === 1 && <text x="2.5" y=".92" fill="#92e3f1" fontSize=".3" textAnchor="middle">↩</text>}
        </g>}
      </svg>
      <button onClick={() => setShown(true)}>Пустить учебный луч из {example.port}</button>
      {shown && <div className="guide-example-result" role="status"><p><strong>{example.outcome}</strong></p><p>{example.explanation}</p></div>}
      <div className="guide-example-actions">
        <button className="secondary-button" disabled={step === 0} onClick={() => go(step - 1)}>Предыдущий пример</button>
        {step < examples.length - 1 ? <button disabled={!shown} onClick={() => go(step + 1)}>Следующий пример</button>
          : <button className="secondary-button" onClick={() => go(0)}>Посмотреть сначала</button>}
      </div>
    </section>
  );
}
