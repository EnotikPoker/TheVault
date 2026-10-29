import { useId, useState, type ReactNode } from 'react';
import './game-guide.css';

interface Props {
  mode: string;
  goal: string;
  steps: string[];
  children: ReactNode;
}

export function GameGuide({ mode, goal, steps, children }: Props) {
  const id = useId();
  const [open, setOpen] = useState(false);
  return (
    <section className="game-guide" aria-labelledby={id}>
      <h2 id={id}>Как играть: {mode}</h2>
      <p className="guide-goal">{goal}</p>
      <ol className="guide-quick-start">{steps.map(step => <li key={step}>{step}</li>)}</ol>
      <details className="guide-details" onToggle={event => setOpen(event.currentTarget.open)}>
        <summary>Открыть инструкцию и учебный пример</summary>
        <div className="guide-content" hidden={!open}>{children}</div>
      </details>
    </section>
  );
}
