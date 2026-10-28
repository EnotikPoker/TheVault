# The Vault

Головоломка о вскрытии сейфа с помощью сканера. На поле 6×6 скрыты три элемента:
игрок запускает лучи с краёв, сравнивает результаты и отмечает предполагаемое
расположение. Цель — найти все элементы с минимальным числом очков.

Есть общий сейф дня, который обновляется в полночь по Москве, и случайные
тренировки. После вскрытия видны расположение элементов и настоящие пути
проверенных лучей. Первоначальная игра с пятью переключателями доступна
в отдельной вкладке.

## Стек и структура

- Go 1.26+, `net/http`, pgx/v5, PostgreSQL 18.
- React, TypeScript, Vite; Node.js 24+ и npm.
- Docker Engine с Compose v2 для локальной базы; GNU Make для общих команд.

```text
backend/
  cmd/server/                 HTTP-сервер и конфигурация
  internal/
    blackbox/                 Поле, лучи, генератор
    blackboxgame/             Сессии, отметки, очки и время
    postgres/                 Хранилище и SQL-миграции
    transport/httpapi/        HTTP-обработчики обоих режимов
    vault/                    Логика переключателей
    session/                  Сессии переключателей в памяти
frontend/src/
  app/                        Выбор игрового режима
  features/blackbox/           Сканер, поле, журнал и правила
  features/vault/              Панель переключателей
  shared/api/                 Клиенты и проверка ответов API
docs/                         Концепция, архитектура, API и дальнейшие этапы
compose.yaml                  PostgreSQL для разработки
```

## Локальный запуск

Из корня проекта:

```sh
make install
make db-up
```

`make db-up` ждёт готовности PostgreSQL. База доступна на `127.0.0.1:5433`,
данные сохраняются в Docker volume `postgres_data`. Учётные данные из
`compose.yaml` предназначены для локальной разработки.

В двух терминалах:

```sh
make dev-backend
```

```sh
make dev-frontend
```

Откройте <http://127.0.0.1:5173>. Vite проксирует `/api` на
<http://127.0.0.1:8080>. Выберите сейф дня или тренировку: с этого момента
начинается таймер. Подробные правила со схемами доступны под игровым полем.

`make dev-backend` задаёт локальный `DATABASE_URL` по умолчанию. Для своей базы:

```sh
DATABASE_URL='postgres://USER:PASSWORD@HOST:PORT/DATABASE?sslmode=disable' make dev-backend
```

Backend требует доступную базу и автоматически применяет миграции при старте.
`HTTP_ADDR` по умолчанию — `127.0.0.1:8080`. Образец переменных —
`backend/.env.example`; файлы `.env` автоматически не загружаются.
При смене адреса backend обновите proxy в `frontend/vite.config.ts`.

Без Make: `npm ci` в `frontend`, `docker compose up -d --wait` в корне,
`DATABASE_URL=... go run ./cmd/server` в `backend` и `npm run dev` в `frontend`.

Остановить базу без удаления прогресса:

```sh
make db-down
```

Сессии сканера, отметки, лучи и ответы переживают перезапуск backend и базы.
Браузер хранит ID сессий отдельно для каждого дня и текущей тренировки.
Сессии режима переключателей по-прежнему находятся в памяти backend.
При недоступном `localStorage` отправка ответа сканера блокируется: браузеру
нужно сохранить идентификатор проверки для безопасного повтора после сбоя.
Аккаунтов и переноса прогресса между устройствами пока нет.

## Проверки и сборка

```sh
make check              # go vet и TypeScript
make test               # Go и Vitest / React Testing Library
make test-integration   # настоящая PostgreSQL, HTTP, параллельные запросы; -race
make build              # backend/bin/server и frontend/dist
make fmt                # форматирование Go
```

Для интеграционных тестов сначала запустите `make db-up` либо задайте
`TEST_DATABASE_URL`. Тесты создают временную схему и удаляют её после завершения;
пользователь базы должен иметь право создавать схемы.

```sh
TEST_DATABASE_URL='postgres://USER:PASSWORD@HOST:PORT/DATABASE?sslmode=disable' make test-integration
```

CI использует отдельный PostgreSQL 18 и выполняет проверки, тесты и сборку.
Для отдельного запуска frontend-тестов: `npm test` из `frontend`.

Сборка создаёт два отдельных артефакта. Для production потребуется раздача
`frontend/dist` и reverse proxy `/api` на Go-сервер. Vite proxy работает только
в разработке; `npm run preview` предназначен для просмотра статической сборки.

## Документация

- [Концепция и правила](docs/concept.md)
- [Архитектура](docs/architecture.md)
- [HTTP API](docs/api.md)
- [Дальнейшие этапы](docs/roadmap.md)
