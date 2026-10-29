import { VaultHistory } from './VaultHistory';
import { VaultPanel } from './VaultPanel';
import { VaultResult } from './VaultResult';
import { VaultGuide } from './VaultGuide';
import { useElapsed } from './useElapsed';
import { useVaultGame } from './useVaultGame';

const operationText = {
  booting: 'Подготовка игры…',
  idle: '',
  creating: 'Создаём новый сейф…',
  restoring: 'Загружаем игру…',
  checking: 'Проверяем комбинацию…',
  recovering: 'Восстанавливаем результат проверки…',
};

export function VaultGame() {
  const game = useVaultGame();
  const elapsed = useElapsed(game.session, game.receivedAt);
  const disabled = game.busy || game.needsSync || game.session?.status === 'opened';

  return (
    <>
      <VaultGuide />
      {game.message && (
        <div className="feedback" role="alert">
          <p>{game.message}</p>
          {game.needsSync && <button disabled={game.busy} onClick={() => void game.retry()}>Повторить загрузку</button>}
        </div>
      )}
      {game.busy && <p className="request-status" role="status">{operationText[game.operation]}</p>}
      {game.session ? (
        <>
          <VaultPanel
            session={game.session}
            selected={game.selected}
            elapsedMs={elapsed}
            disabled={disabled}
            checking={game.operation === 'checking' || game.operation === 'recovering'}
            onSelect={game.select}
            onCheck={() => void game.check()}
          />
          {game.session.status === 'opened' && <VaultResult session={game.session} />}
          <VaultHistory history={game.session.history} disabled={disabled} onSelect={game.select} />
          <div className="game-actions">
            <button className="secondary-button" disabled={game.busy} onClick={() => void game.startNew()}>Новый сейф</button>
          </div>
        </>
      ) : !game.busy && (
        <section className="start-card" aria-labelledby="start-title">
          <h2 id="start-title">Пять переключателей. Три системы.</h2>
          <p>Проверяйте комбинации и сравнивайте результаты. Сейф откроется, когда все индикаторы станут зелёными.</p>
          <p className="muted">Таймер начнётся после нажатия кнопки.</p>
          <button onClick={() => void game.startNew()}>{game.needsSync ? 'Начать новую игру' : 'Начать игру'}</button>
        </section>
      )}
    </>
  );
}
