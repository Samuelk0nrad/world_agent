# Copilot Instructions

## Build, test, and lint commands

### Root workspace
- Initialize submodules before working across the full repo:
  - `git submodule update --init --recursive`

### `agent-backend` (Go / Gin)
- Setup:
  - `cd agent-backend && cp example.env .env`
- Run locally:
  - `go run ./cmd/server`
- Build:
  - `go build ./cmd/server`
- Test all:
  - `go test ./...`
- Run a single test:
  - `go test ./internal/server -run TestAgentRunEndpoint`

### `mobile-app` (Expo / React Native)
- Setup:
  - `cd mobile-app && cp example.env .env && npm install`
- Start dev server:
  - `npm run start`
- Platform runs:
  - `npm run android`
  - `npm run ios`
  - `npm run web`
- Type-check:
  - `npm run typecheck`

### `docker-ory` submodule (`docker-ory/example-next-app`)
When changing `docker-ory/example-next-app`, keep parity with the existing repo instruction file:
- `cd docker-ory/example-next-app`
- Build:
  - `bun run build`
- Lint:
  - `bun run lint`
- Full tests:
  - `bun run test:all`
- Run a single test file:
  - `bun run test -- test/unit/utils.test.ts`
- Focused suites:
  - `bun run test:unit`
  - `bun run test:integration`

## High-level architecture

WorldAgent is a multi-part workspace with a mobile-first client, a Go agent backend, and an auth/OIDC stack submodule.

- `mobile-app` is an Expo Router app with three tabs (`Assistant`, `Extensions`, `Settings`). Shared runtime config (`backendUrl`, optional `googleAccessToken`) lives in `AppConfigProvider` and is consumed by all tabs.
- `mobile-app/src/api/client.ts` is the only network layer; it maps UI actions to backend routes (`/healthz`, `/v1/extensions`, `/v1/memory`, `/v1/agent/run`).
- `agent-backend/internal/server/router.go` wires the runtime: file-backed memory store + extension registry + connector registry + optional Gemini text generation connector.
- `agent-backend/internal/agent/loop.go` runs the orchestration loop: persists user input, conditionally invokes connectors (web-search/email), builds observations, generates final response, and persists assistant output.
- Persistent memory is JSONL-backed (`agent-backend/internal/store/file_store.go`) with monotonic `sequence`; `/v1/memory?since=<n>` supports incremental sync for clients.
- `docker-ory` is a git submodule for OAuth/OIDC (Hydra + Kratos + Next.js). It is separate from the Go/mobile runtime but provides the identity stack used by related flows.

## Key conventions in this codebase

- Backend config is `.env`-centric and loaded via Viper from the current working directory (`agent-backend/internal/config/env.go`). Missing `.env` is treated as an error path in config loading tests.
- Environment variables override `.env` values (tested behavior in `agent-backend/internal/config/env_test.go`).
- Connectors are registered by normalized lowercase IDs (`agent-backend/internal/connectors/registry.go`); keep IDs stable (`web-search`, `gmail`, `gemini`) and lowercase when adding connectors.
- Unavailable connector pattern is intentional: if credentials/config are missing, router registers unavailable connectors that return explicit errors instead of silent no-ops.
- Gmail access token propagation is end-to-end by request: mobile Settings stores token -> API client sends `googleAccessToken` in `/v1/agent/run` body -> backend also supports `X-Google-Access-Token` fallback header.
- Extension toggles are runtime state in the backend extension registry (`/v1/extensions` + `PATCH /v1/extensions/:id`); UI should treat backend as source of truth for enabled/disabled status.
- Memory API response shape and field naming are part of contract (`entries`, `latest_sequence`), and `since` must be a non-negative integer.
