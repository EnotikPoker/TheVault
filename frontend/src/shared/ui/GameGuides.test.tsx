import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from '../../app/App';
import { Rules } from '../../features/blackbox/Rules';
import { VaultGame } from '../../features/vault/VaultGame';
import * as scanner from '../api/blackbox';
import * as vault from '../api/vaults';
import { dailyInfo } from '../../test/blackboxFixtures';
import { checkedSession, sessionId } from '../../test/fixtures';

vi.mock('../api/blackbox', async original => ({
  ...await original<typeof scanner>(), getDaily: vi.fn(), createBlackBox: vi.fn(), getBlackBox: vi.fn(),
  scanBlackBox: vi.fn(), markBlackBox: vi.fn(), answerBlackBox: vi.fn(),
}));
vi.mock('../api/vaults', async original => ({
  ...await original<typeof vault>(), createVault: vi.fn(), getVault: vi.fn(), checkVault: vi.fn(),
}));
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(scanner.getDaily).mockResolvedValue(dailyInfo);
  vi.mocked(vault.getVault).mockResolvedValue(checkedSession());
});

describe('instructions in both modes', () => {
  it('puts the goal and instructions before the game controls without creating sessions', async () => {
    const user = userEvent.setup();
    render(<App />, { reactStrictMode: true });
    expect(screen.getByRole('heading', { name: 'Как играть: Сканер' })).toBeVisible();
    const summary = screen.getByText('Открыть инструкцию и учебный пример');
    expect(summary.closest('details')).not.toHaveAttribute('open');
    await user.click(summary);
    expect(screen.getByRole('region', { name: 'Учебный макет сканера' })).toBeVisible();
    await user.click(summary);
    expect(summary.closest('details')).not.toHaveAttribute('open');
    await user.click(screen.getByRole('button', { name: 'Переключатели' }));
    expect(screen.getByRole('heading', { name: 'Как играть: Переключатели' })).toBeVisible();
    expect(screen.getByText(/Найдите положение пяти переключателей/)).toBeVisible();
    await user.click(screen.getByText('Открыть инструкцию и учебный пример'));
    expect(screen.getByRole('region', { name: 'Учебный пример переключателей' })).toBeVisible();
    expect(scanner.createBlackBox).not.toHaveBeenCalled();
    expect(vault.createVault).not.toHaveBeenCalled();
  });

  it('demonstrates all five ray cases and resets each path before the next case', async () => {
    const user = userEvent.setup();
    render(<Rules />);
    await user.click(screen.getByText('Открыть инструкцию и учебный пример'));
    const lesson = within(screen.getByRole('region', { name: 'Учебный макет сканера' }));
    const ports = ['W5', 'N3', 'W2', 'S3', 'N2'];
    const results = ['W5 → E5: выход с другой стороны', 'N3 → поглощение (×)', 'W2 → N1: поворот на 90°', 'S3 → возврат (↩)', 'N2 → возврат (↩)'];
    for (const [index, port] of ports.entries()) {
      expect(lesson.queryByRole('status')).not.toBeInTheDocument();
      if (index < 4) expect(lesson.getByRole('button', { name: 'Следующий пример' })).toBeDisabled();
      await user.click(lesson.getByRole('button', { name: `Пустить учебный луч из ${port}` }));
      expect(lesson.getByRole('status')).toHaveTextContent(results[index]!);
      expect(lesson.getByRole('img')).toHaveAccessibleName(`Учебный луч: ${results[index]}`);
      if (index === 2) expect(document.querySelector('polyline')).toHaveAttribute('points', '0.5,2.5 1.5,2.5 1.5,1.5 1.5,0.5');
      if (index < 4) await user.click(lesson.getByRole('button', { name: 'Следующий пример' }));
    }
    expect(document.querySelector('polyline')).not.toBeInTheDocument();
    await user.click(lesson.getByRole('button', { name: 'Посмотреть сначала' }));
    expect(lesson.getByText('Пример 1 из 5')).toBeInTheDocument();
    expect(lesson.queryByRole('status')).not.toBeInTheDocument();
    expect(scanner.scanBlackBox).not.toHaveBeenCalled();
    expect(scanner.markBlackBox).not.toHaveBeenCalled();
    expect(scanner.answerBlackBox).not.toHaveBeenCalled();
  });

  it('walks through a complete switch example without spending real attempts or changing the panel', async () => {
    const user = userEvent.setup();
    localStorage.setItem('the-vault.session-id', sessionId);
    render(<VaultGame />);
    await waitFor(() => expect(screen.getByTestId('attempt-count')).toHaveTextContent('1'));
    const panel = screen.getByRole('region', { name: 'Панель сейфа' });
    const switchesBefore = within(panel).getAllByRole('switch').map(e => (e as HTMLInputElement).checked);
    await user.click(screen.getByText('Открыть инструкцию и учебный пример'));
    const lesson = within(screen.getByRole('region', { name: 'Учебный пример переключателей' }));
    const passedCounts = [0, 2, 2, 2, 3];
    for (let index = 0; index < 5; index++) {
      expect(lesson.queryByRole('status')).not.toBeInTheDocument();
      await user.click(lesson.getByRole('button', { name: 'Проверить учебную комбинацию' }));
      expect(lesson.queryAllByText('✓ ПРОЙДЕНО')).toHaveLength(passedCounts[index]!);
      if (index < 4) {
        expect(lesson.queryByText('Какие правила были в этом примере')).not.toBeInTheDocument();
        await user.click(lesson.getByRole('button', { name: 'Следующий шаг' }));
      }
    }
    expect(lesson.getByText('Какие правила были в этом примере')).toBeVisible();
    expect(lesson.getByText(/S1 и S2 в разных положениях/)).toBeVisible();
    expect(vault.checkVault).not.toHaveBeenCalled();
    expect(vault.createVault).not.toHaveBeenCalled();
    expect(screen.getByTestId('attempt-count')).toHaveTextContent('1');
    expect(within(panel).getAllByRole('switch').map(e => (e as HTMLInputElement).checked)).toEqual(switchesBefore);
    expect(localStorage.getItem('the-vault.session-id')).toBe(sessionId);
    await user.click(lesson.getByRole('button', { name: 'Пройти пример заново' }));
    expect(lesson.getByText('Шаг 1 из 5')).toBeVisible();
    expect(lesson.queryByText('Какие правила были в этом примере')).not.toBeInTheDocument();
  });
});
