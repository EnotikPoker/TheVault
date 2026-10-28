import { VaultGame } from '../features/vault/VaultGame';

export function App() {
  return (
    <main className="app">
      <header>
        <p className="eyebrow">ЛОГИЧЕСКАЯ ГОЛОВОЛОМКА</p>
        <h1>THE VAULT</h1>
        <p className="intro">Исследуйте систему. Найдите закономерность. Откройте сейф.</p>
      </header>
      <VaultGame />
    </main>
  );
}
