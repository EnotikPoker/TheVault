# The Vault

Логическая игра об исследовании неизвестной системы. Игрок меняет переключатели,
проверяет комбинации и по реакции индикаторов восстанавливает правила сейфа.
Цель — открыть его за минимальное число попыток.

## Состояние проекта

Реализованы игровое ядро на Go, сессии и HTTP API. Можно создать сейф, проверять
комбинации и получить историю попыток, время и объяснения после открытия.
Сессии хранятся в памяти и сбрасываются при перезапуске сервера.

Панель на React + TypeScript пока не подключена к игровому API:
переключатели работают локально, кнопка проверки отключена.

## Стек

- Backend: Go 1.26+, стандартная библиотека `net/http`.
- Frontend: React, TypeScript, Vite, HTML/CSS.
- Инструменты: Node.js 24+, npm, GNU Make (необязательно).

## Структура

```text
TheVault/
├── backend/
│   ├── cmd/server/                 # Точка входа HTTP-сервера
│   ├── internal/
│   │   ├── transport/httpapi/      # Маршруты и HTTP-обработчики
│   │   ├── session/                # Сессии, история и хранилище в памяти
│   │   └── vault/                  # Правила, генератор и объяснения
│   ├── .env.example
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── app/                    # React-компонент App
│   │   ├── features/vault/         # React-компонент VaultPanel
│   │   ├── shared/api/             # Обращения к backend
│   │   ├── main.tsx
│   │   └── styles.css
│   ├── package.json
│   ├── package-lock.json
│   └── vite.config.ts
├── docs/
│   ├── concept.md                  # Концепция и требования к механике
│   ├── architecture.md             # Границы компонентов
│   ├── api.md                      # HTTP API
│   └── roadmap.md                  # Этапы реализации
├── .github/workflows/ci.yml
└── Makefile
```

## Локальный запуск

Установите Go и Node.js, затем из корня проекта:

```sh
make install
```

В первом терминале:

```sh
make dev-backend
```

Во втором терминале:

```sh
make dev-frontend
```

Откройте <http://127.0.0.1:5173>. Vite проксирует `/api` на
<http://127.0.0.1:8080>, поэтому отдельная настройка CORS для разработки не нужна.

Без Make те же команды: `npm ci` в `frontend`, `go run ./cmd/server` в
`backend` и `npm run dev` в `frontend`.

Проверка API:

```sh
curl http://127.0.0.1:8080/api/health
# {"status":"ok"}
```

Создание игры:

```sh
curl -X POST http://127.0.0.1:8080/api/v1/vaults
```

Подставьте `id` из ответа вместо `SESSION_ID` для проверки комбинации:

```sh
curl http://127.0.0.1:8080/api/v1/vaults/SESSION_ID/checks \
  -H 'Content-Type: application/json' \
  -d '{"switches":[true,false,true,false,false]}'
```

Состояние и полная история доступны через `GET /api/v1/vaults/SESSION_ID`.
Форматы ответов и ошибок описаны в [документации API](docs/api.md).

Сервер читает `HTTP_ADDR` из окружения, значение по умолчанию — `127.0.0.1:8080`.
Файл `backend/.env.example` служит образцом; `.env` автоматически не загружается.
При смене адреса backend обновите `server.proxy` в `frontend/vite.config.ts`.

## Проверки и сборка

```sh
make check    # go vet и проверка типов TypeScript
make test     # go test
make build    # backend/bin/server и frontend/dist/
make fmt      # форматирование Go
```

Проверка параллельных вызовов с детектором гонок:

```sh
cd backend && go test -race ./...
```

`make build` создаёт два отдельных артефакта. Для production потребуется раздача
`frontend/dist` и проксирование `/api` на Go-сервер. Vite proxy работает только
в режиме разработки; `npm run preview` предназначен для просмотра статической сборки.

## Документация

- [Концепция игры](docs/concept.md)
- [Архитектура](docs/architecture.md)
- [HTTP API](docs/api.md)
- [План реализации](docs/roadmap.md)
