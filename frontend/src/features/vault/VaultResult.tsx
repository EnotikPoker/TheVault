import { useEffect, useRef } from 'react';
import type { OpenedSession } from '../../shared/api/vaults';
import { formatElapsed } from './time';

export function VaultResult({ session }: { session: OpenedSession }) {
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => { heading.current?.focus(); }, [session.id]);

  return (
    <section className="result" aria-labelledby="result-title">
      <h2 id="result-title" ref={heading} tabIndex={-1}>Сейф открыт</h2>
      <p>Попытки: {session.attempts} · Время: {formatElapsed(session.elapsedMs)}</p>
      <h3>Как работали системы</h3>
      <ul className="explanations">
        {session.explanations.map(({ id, condition, observation }) => (
          <li key={id}>
            <h4>{id.toUpperCase()}</h4>
            <p>{condition}</p>
            <p className="muted">{observation}</p>
          </li>
        ))}
      </ul>
    </section>
  );
}
