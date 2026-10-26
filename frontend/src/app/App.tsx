import { useEffect, useState } from 'react';
import { VaultPanel } from '../features/vault/VaultPanel';
import { checkHealth } from '../shared/api/health';

type ConnectionStatus = 'connecting' | 'connected' | 'unavailable';

const connectionMessages: Record<ConnectionStatus, string> = {
  connecting: 'Соединение с сервером…',
  connected: 'Сервер подключён',
  unavailable: 'Сервер недоступен. Запустите backend и обновите страницу.',
};

export function App() {
  const [connection, setConnection] = useState<ConnectionStatus>('connecting');

  useEffect(() => {
    const controller = new AbortController();

    void checkHealth(controller.signal).then(
      () => {
        if (!controller.signal.aborted) setConnection('connected');
      },
      () => {
        if (!controller.signal.aborted) setConnection('unavailable');
      },
    );

    return () => controller.abort();
  }, []);

  return (
    <main className="app">
      <header>
        <p className="eyebrow">ЛОГИЧЕСКАЯ ГОЛОВОЛОМКА</p>
        <h1>THE VAULT</h1>
        <p className="intro">Исследуйте систему. Найдите закономерность. Откройте сейф.</p>
      </header>
      <VaultPanel />
      <p className="connection" role="status">{connectionMessages[connection]}</p>
    </main>
  );
}
