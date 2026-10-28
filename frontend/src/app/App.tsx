import { VaultGame } from '../features/vault/VaultGame';
import { BlackBoxGame } from '../features/blackbox/BlackBoxGame';
import { useState } from 'react';

export function App() {
  const [mode, setMode] = useState<'scanner' | 'switches'>('scanner');
  return (
    <main className="app">
      <header>
        <p className="eyebrow">ЛОГИЧЕСКАЯ ГОЛОВОЛОМКА</p>
        <h1>THE VAULT</h1>
        <p className="intro">Исследуйте механизм. Проверьте гипотезу. Откройте сейф.</p>
      </header>
      <nav className="game-tabs" aria-label="Режим игры">
        <button className="secondary-button" aria-pressed={mode === 'scanner'} onClick={() => setMode('scanner')}>Сканер</button>
        <button className="secondary-button" aria-pressed={mode === 'switches'} onClick={() => setMode('switches')}>Переключатели</button>
      </nav>
      {mode === 'scanner' ? <BlackBoxGame /> : <VaultGame />}
    </main>
  );
}
