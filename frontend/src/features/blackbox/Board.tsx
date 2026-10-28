import { useRef, useState, type KeyboardEvent } from 'react';
import { cellKey, type BlackBoxSession, type Cell, type Mark, type Port, type Scan } from '../../shared/api/blackbox';

const sides = { top: 'N', right: 'E', bottom: 'S', left: 'W' };
export const portName = (p: Port) => `${sides[p.side]}${p.index + 1}`;
export const cellName = (c: Cell) => `${String.fromCharCode(65 + c.col)}${c.row + 1}`;
export function scanText(scan: Scan) {
  return `${portName(scan.entry)} → ${scan.result.kind === 'hit' ? 'поглощение' : scan.result.kind === 'reflection' ? 'возврат' : portName(scan.result.exit!)}`;
}
const samePort = (a: Port, b?: Port) => b !== undefined && a.side === b.side && a.index === b.index;

interface Props {
  session: BlackBoxSession;
  tool: Mark['state'];
  disabled: boolean;
  selectedScan: number | null;
  onScan: (port: Port) => void;
  onMark: (cell: Cell) => void;
}

export function Board({ session, tool, disabled, selectedScan, onScan, onMark }: Props) {
  const root = useRef<HTMLDivElement>(null);
  const [focused, setFocused] = useState(9);
  const opened = session.status === 'opened';
  function navigate(event: KeyboardEvent<HTMLDivElement>) {
    const target = event.target as HTMLButtonElement;
    const offsets: Record<string, number> = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -8, ArrowDown: 8 };
    const offset = offsets[event.key];
    if (!offset || target.dataset.position === undefined) return;
    event.preventDefault();
    let position = Number(target.dataset.position) + offset;
    while (position >= 0 && position < 64) {
      if (Math.abs(offset) === 1 && Math.floor(position / 8) !== Math.floor(Number(target.dataset.position) / 8)) return;
      const next = root.current?.querySelector<HTMLButtonElement>(`button[data-position="${position}"]:not(:disabled)`);
      if (next) { next.focus(); return; }
      position += offset;
    }
  }
  const trace = session.scans.find(s => s.number === selectedScan);
  return (
    <div className="bb-board-wrap">
      <div className="bb-board" ref={root} onKeyDown={navigate} role="group" aria-label="Поле сканера 6 на 6. Стрелки перемещают фокус, Enter выполняет действие.">
        {Array.from({ length: 64 }, (_, position) => {
          const row = Math.floor(position / 8) - 1, col = position % 8 - 1;
          if ((row === -1 || row === 6) && (col === -1 || col === 6)) return <span key={position} aria-hidden="true" />;
          if (row === -1 || row === 6 || col === -1 || col === 6) {
            const port: Port = row === -1 ? { side: 'top', index: col } : row === 6 ? { side: 'bottom', index: col }
              : col === -1 ? { side: 'left', index: row } : { side: 'right', index: row };
            const scan = session.scans.find(s => samePort(port, s.entry) || samePort(port, s.result.exit));
            const symbol = !scan ? '·' : scan.result.kind === 'hit' ? '×' : scan.result.kind === 'reflection' ? '↩' : String(scan.number);
            return <button key={position} className="bb-port" data-position={position} data-result={scan?.result.kind}
              tabIndex={focused === position ? 0 : -1} onFocus={() => setFocused(position)}
              data-selected={scan?.number === selectedScan} disabled={disabled || opened}
              aria-label={`Порт ${portName(port)}. ${scan ? `${scanText(scan)}. Повтор бесплатно` : 'Сканировать: +1 очко'}`}
              onClick={() => onScan(port)}><span>{portName(port)}</span><b aria-hidden="true">{symbol}</b></button>;
          }
          const cell = { row, col };
          const mark = session.marks.find(m => cellKey(m) === cellKey(cell));
          const solution = session.solution?.some(c => cellKey(c) === cellKey(cell));
          const text = opened ? solution ? 'элемент' : 'пусто' : mark?.state === 'suspected' ? 'предполагаемый элемент' : mark?.state === 'empty' ? 'отмечено пусто' : 'без отметки';
          return <button key={position} className="bb-cell" data-position={position} data-mark={opened ? solution ? 'solution' : '' : mark?.state}
            tabIndex={focused === position ? 0 : -1} onFocus={() => setFocused(position)}
            disabled={disabled || opened} aria-label={`Клетка ${cellName(cell)}: ${text}. ${tool === 'suspected' ? 'Отметить элемент' : 'Отметить пустую клетку'}`}
            aria-pressed={opened ? Boolean(solution) : mark?.state === tool} onClick={() => onMark(cell)}>
            <small aria-hidden="true">{cellName(cell)}</small><span aria-hidden="true">{opened ? solution ? '●' : '' : mark?.state === 'suspected' ? '●' : mark?.state === 'empty' ? '×' : ''}</span>
          </button>;
        })}
        {opened && trace?.path && <svg className="bb-trace" viewBox="0 0 8 8" role="img" aria-label={`Путь луча: ${scanText(trace)}`}>
          <polyline points={trace.path.map(c => `${c.col + 1.5},${c.row + 1.5}`).join(' ')} />
          {trace.path.map((c, i) => <circle key={i} cx={c.col + 1.5} cy={c.row + 1.5} r={i === 0 ? 0.12 : 0.06} />)}
        </svg>}
      </div>
      <p className="bb-legend"><span>× поглощение</span><span>↩ возврат</span><span>одинаковый номер — пара портов</span></p>
    </div>
  );
}
