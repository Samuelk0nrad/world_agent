# The World Game

## running this locally (dev)

1. clone the repo: `git clone git@github.com:Samuelk0nrad/world_game.git WorldGame`
2. go into the repo: `cd WorldGame`
3. clone the subrepos: `git submodule init && git submodule update`

## New agent scaffolding

- `mobile-app/`: React Native (Expo) chat-first shell with tabs: Assistant, Extensions, Settings
- `agent-backend/`: Golang (Gin) backend skeleton with:
  - `GET /healthz`
  - `GET /v1/extensions`
  - `PATCH /v1/extensions/:id`
  - `GET /v1/memory` (supports incremental sync with `?since=<sequence>`)
- `POST /v1/memory`
- `POST /v1/agent/run`
  - `GET /v1/logs/events` (for UI log streaming/polling)

### Backend quick start

Backend internals are now split for extension without breaking API clients:
- `internal/ai`: provider/model contracts + registry (Gemini is the default provider today).
- `internal/agentloop`: evented turn runner (message/tool lifecycle, hooks, deterministic event sequencing).
- `internal/agent`: runtime orchestration that maps loop/AI behavior back to the stable HTTP response contract.

Gemini integration is available by default for final agent responses. Configure it before running:

```sh
cd agent-backend
cp example.env .env
go run ./cmd/server
```

Default port: `8088` (override with `AGENT_BACKEND_PORT`).
Memory file path: `./data/memory.jsonl` (override with `AGENT_MEMORY_FILE`).
Backend env loading uses `github.com/spf13/viper` and always reads `./.env`.
Memory entries include `id`, `source`, `content`, `created_at`, and `sequence` for sync-friendly pulls.
LLM connector is disabled by default. Set `AGENT_LLM_CONNECTOR=gemini` to enable Gemini first.
As additional providers/connectors are registered, `AGENT_LLM_CONNECTOR` remains the extension point for selecting them.
If Gemini is requested but `GEMINI_API_KEY` is missing, `/v1/agent/run` returns an explicit configuration error.
Gemini upstream/transport failures are also surfaced as explicit `/v1/agent/run` connector errors.
Web search uses a SerpAPI connector. Configure it with:
- `SERPAPI_API_KEY` (required)
- `SERPAPI_ENGINE` (optional, defaults to `google`)
If web search is invoked without `SERPAPI_API_KEY`, `/v1/agent/run` returns an explicit connector configuration error.
Gmail connector primitives are wired for email extension read/send flows. Configure OAuth app values:
- `GOOGLE_CLIENT_ID` (required)
- `GOOGLE_CLIENT_SECRET` (required)
- `GOOGLE_REDIRECT_URL` (required)
Pass a user Gmail access token per `/v1/agent/run` request via:
- JSON field `googleAccessToken`
- or header `X-Google-Access-Token`
If OAuth config or token is missing, `/v1/agent/run` returns an explicit Gmail connector error.
Capability gates default to safe mode (`email`, `mobile-sensors`, `screen-capture`, `audio-capture` disabled).
Enable only the capabilities you need with:
- `AGENT_CAPABILITY_EMAIL=true`
- `AGENT_CAPABILITY_MOBILE_SENSORS=true`
- `AGENT_CAPABILITY_SCREEN_CAPTURE=true`
- `AGENT_CAPABILITY_AUDIO_CAPTURE=true`

Comprehensive backend logging is configurable via env:
- `AGENT_LOG_LEVEL=debug|info|warn|error`
- `AGENT_LOG_FORMAT=json|text`
- `AGENT_LOG_EVENTS_ENABLED=true|false`
- `AGENT_LOG_API_ENABLED=true|false`
- `AGENT_LOG_INCLUDE_PAYLOAD=true|false` (logs message/prompt/response payloads)
- `AGENT_LOG_EVENT_BUFFER=<number>` (ring buffer size)

`POST /v1/agent/run` keeps the existing contract (`result.reply`, `result.steps`) while internals evolve.
New tools/modules/providers should be added behind this contract boundary.

For building a log UI, query:
- `GET /v1/logs/events?since=<sequence>&limit=<n>&type=<event_type>`

### Backend Docker Compose

```sh
cd agent-backend
cp example.env .env
docker compose up --build
```

The backend is exposed on `http://localhost:8088`.
Compose passes and mounts `./.env` into the container, so update `agent-backend/.env` before starting.

### Mobile quick start

```sh
cd mobile-app
cp example.env .env
npm install
npm run start
```

The mobile app defaults to `http://localhost:8088`, but you can set API values directly in the **Settings** tab:
- Backend URL
- Optional Google access token (used for Gmail connector tests)

### Monitoring web quick start

```sh
cd monitoring-web
npm install
npm run dev
```

Open `http://localhost:3000`, set **Backend URL** in the UI (default `http://localhost:8088`), then click **Start** or **Poll now**.
The monitor polls backend logs from `GET /v1/logs/events?since=<n>&limit=<n>` (through the app route `GET /api/logs/events`).
