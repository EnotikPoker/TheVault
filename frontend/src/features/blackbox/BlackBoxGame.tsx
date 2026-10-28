import { useState } from 'react';
import type { Mark } from '../../shared/api/blackbox';
import { useElapsed } from '../vault/useElapsed';
import { formatElapsed } from '../vault/time';
import { Board, scanText } from './Board';
import { Rules } from './Rules';
import { useBlackBox } from './useBlackBox';
import './blackbox.css';

export function BlackBoxGame() {
  const game = useBlackBox();
  const [tool, setTool] = useState<Mark['state']>('suspected');
  const [selected, setSelected] = useState<{ id: string; scan: number } | null>(null);
  const state = game.session;
  const elapsed = useElapsed(state, game.receivedAt);
  const disabled = game.busy || game.needsSync;
  const opened = state?.status === 'opened';
  const count = state?.marks.filter(m => m.state === 'suspected').length ?? 0;
  return (
    <section className="blackbox" aria-label="Сканирование механизма">
      <div className="bb-heading"><div><p className="eyebrow">BLACK BOX · 6 × 6</p><h2>Что скрыто внутри?</h2></div><span className="bb-chip">3 элемента</span></div>
      <p className="intro">Запускайте лучи по краям сейфа. По их реакции найдите три скрытых элемента.</p>
      <div className="bb-modes" aria-label="Тип головоломки">
        <button className={state?.mode === 'daily' ? '' : 'secondary-button'} disabled={disabled} onClick={() => void game.start('daily')}>
          Сейф дня{game.daily ? ` · ${game.daily.date.slice(8)}.${game.daily.date.slice(5, 7)}` : ''}
        </button>
        <button className={state?.mode === 'practice' ? '' : 'secondary-button'} disabled={disabled} onClick={() => void game.start('practice')}>Тренировка</button>
      </div>
      <p className="muted">Сейф дня общий для всех, новый — в 00:00 по Москве. В тренировке — случайная раскладка.</p>
      {game.message && <div className="feedback" role="alert"><p>{game.message}</p></div>}
      {game.needsSync && <div className="feedback"><p>Есть незавершённый запрос. Восстановление не начислит повторный штраф.</p><button disabled={game.busy} onClick={() => void game.retry()}>Восстановить состояние</button></div>}
      {game.busy && <p role="status" className="request-status">Связь со сканером…</p>}
      {!state && !game.busy && !game.needsSync && <p className="muted">Выберите сейф дня или тренировку. Таймер начнётся при первом запуске.</p>}
      {state && <>
        {state.mode === 'daily' && game.daily && state.date !== game.daily.date && <p className="feedback">Вы решаете сейф за {state.date}. Его можно закончить или перейти к сегодняшнему.</p>}
        <section className="bb-panel" aria-labelledby="scanner-title" aria-busy={game.busy}>
          <div className="panel-heading"><h3 id="scanner-title">{state.mode === 'daily' ? `СЕЙФ ДНЯ · ${state.date}` : 'ТРЕНИРОВКА'}</h3><span>{opened ? 'ОТКРЫТ' : 'СКАНЕР ГОТОВ'}</span></div>
          <dl className="bb-metrics"><div><dt>Очки</dt><dd>{state.score}</dd></div><div><dt>Лучи</dt><dd>{state.scanCount}</dd></div><div><dt>Ошибки</dt><dd>{state.wrongAnswers} × 5</dd></div><div><dt>Время</dt><dd><time>{formatElapsed(elapsed)}</time></dd></div></dl>
          {!opened && <div className="bb-tools" role="group" aria-label="Инструмент отметки">
            <button className="secondary-button" aria-pressed={tool === 'suspected'} onClick={() => setTool('suspected')}>● Элемент ({count}/3)</button>
            <button className="secondary-button" aria-pressed={tool === 'empty'} onClick={() => setTool('empty')}>× Пусто</button>
          </div>}
          <Board session={state} tool={tool} disabled={disabled} selectedScan={selected?.id === state.id ? selected.scan : null}
            onScan={p => void game.scan(p)} onMark={c => void game.mark(c, tool)} />
          {!opened && <><p className="muted">Нажатие на клетку ставит отметку, повторное — снимает. Отметки бесплатны и сохраняются.</p>
            <button className="check-button" disabled={disabled || count !== 3} onClick={() => void game.answer()}>Проверить расположение · {count}/3</button>
            <p className="check-note">За неверный ответ +5 очков. Меньше очков — лучше.</p></>}
        </section>
        {opened && <section className="result" aria-labelledby="bb-result-title"><h2 id="bb-result-title">Механизм раскрыт</h2><p>{state.score} очков · {state.scanCount} сканирований · {formatElapsed(state.elapsedMs)}</p><p>На поле показано настоящее расположение. Выберите сканирование в журнале, чтобы увидеть, как прошёл луч.</p></section>}
        <section className="history" aria-labelledby="bb-history-title"><h3 id="bb-history-title">Журнал сканирований</h3>
          {state.scans.length ? <ol className="bb-log">{state.scans.map(scan => <li key={scan.number}>
            <span className="bb-log-number">{scan.number}</span><span>{scanText(scan)}</span>
            {opened && <button className="secondary-button" aria-pressed={selected?.id === state.id && selected.scan === scan.number}
              onClick={() => setSelected(selected?.id === state.id && selected.scan === scan.number ? null : { id: state.id, scan: scan.number })}>Путь луча {scan.number}</button>}
          </li>)}</ol> : <p className="muted">Нажмите на любой порт по краю поля. Новый луч стоит 1 очко.</p>}
        </section>
        {state.mode === 'practice' && <div className="game-actions"><button className="secondary-button" disabled={disabled} onClick={() => void game.start('practice', true)}>Новая тренировка</button></div>}
      </>}
      <Rules />
    </section>
  );
}
