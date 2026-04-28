# Knowvia

Manus-like research agent workspace built alongside the existing `yuque-rag`
apps. This new workspace keeps the current Python/React Native product intact
and introduces a new stack:

- `app/`: Expo mobile client, locked to Node 20
- `server/`: Go API + background execution services
- `infra/`: local Postgres/pgvector + Redis
- `docs/`: architecture, event protocol, and environment conventions

## Goals

- Preserve Yuque knowledge retrieval and structured answer generation
- Add long-running task execution with visible planning and step progress
- Deliver structured reports instead of plain chat replies
- Keep the first version personal-use friendly and easy to run locally

## Local Development

### Web

```bash
cd quickque-agent/web
nvm use
npm install
npm run dev
```

This serves the standalone official website at `http://127.0.0.1:4174`.

### App

```bash
cd quickque-agent/app
nvm use
npm install
npm run dev
```

Node version is pinned to the 20.x line.
The Expo app reuses the original `mobile/src/asserts/logo.png` brand icon.

### Android Packaging

```bash
cd quickque-agent/app
nvm use
npm install
npm run build:android:release
```

Available build commands:

- `npm run build:android`
- `npm run build:android:debug`
- `npm run build:android:release`
- `npm run build:android:aab`

The packaged files are copied to `quickque-agent/app/dist/android/`.

### Server

```bash
cd quickque-agent/server
./run_server.sh
```

`server/.env` 已经默认指向你本机的 PostgreSQL：

```bash
QQA_POSTGRES_DSN=postgres://chenhaibin@127.0.0.1:5432/quickque_agent?sslmode=disable
```

服务启动时会自动读取 `server/.env` 并应用 schema。要切回内存模式，只需把 `QQA_STORE_BACKEND` 改成 `memory`。

If you enable `QQA_PYTHON_PROXY_BASE_URL` in `server/.env`, you also need the
legacy Python backend:

```bash
cd quickque-agent/llm
./run_server.sh
```

That service listens on `http://127.0.0.1:8000` and handles knowledge sync plus
forwarded knowledge chat for the Go API. Go -> Python uses
`QQA_PYTHON_PROXY_TOKEN` for internal auth.

### Run Together

Open three terminals for the default stack:

```bash
cd quickque-agent/server
./run_server.sh
```

```bash
cd quickque-agent/app
nvm use
npm install
npm run dev
```

Open a fourth terminal only if `QQA_PYTHON_PROXY_BASE_URL` is enabled:

```bash
cd quickque-agent/llm
./run_server.sh
```
