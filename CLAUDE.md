# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

**Backend:**
```bash
go run ./cmd/litetask          # Run dev server on :8080
go test ./...                  # Run all tests
go test -race ./...            # Run all tests with race detector (used in CI)
go test ./internal/store/...   # Run single package tests
golangci-lint run --timeout=5m # Lint Go code
```

**Frontend:**
```bash
cd web && npm run dev          # Dev server on :5173 (proxies /api to :8080)
cd web && npm run build        # Production build → web/dist/
cd web && npm run lint         # ESLint
```

**Docker:**
```bash
docker compose up --build      # Full stack via docker-compose.yaml
make docker-build              # Build image only
```

## Architecture

LiteTask is a self-hosted task board: Go backend + SQLite + React frontend, deployed as a single Docker image.

### Layers

```
web/src/             React SPA (Vite + Ant Design) — served as static files in production
internal/httpapi/    HTTP API server + session auth
internal/store/      SQLite data layer (all DB access)
internal/botcore/    Shared bot command handler (Commander) used by tgbot and discordbot
internal/tgbot/      Optional Telegram bot (long-polling)
internal/discordbot/ Optional Discord bot (slash commands)
internal/config/     Shared env-var helper (EnvOrDefault)
cmd/litetask/        Entry point — wires everything together
```

### Key design points

- **No router library** — `net/http` with manual path/method dispatch in `server.go`
- **Auth** — HMAC-SHA256 signed cookie (`auth`). Token encodes `userID:role:expiry`. Validated on every request by `requireUser` middleware.
- **Role-based access** — `admin` sees all projects; `user` is filtered to rows in `user_projects` join table; `blocked` is rejected at middleware (403).
- **Store interface** — `internal/store/interface.go` defines `store.Storer`; `Server` and `Commander` depend on the interface, not `*store.Store`. All Store methods take `context.Context` as first param.
- **Error responses** — all API errors are JSON `{"error":"..."}` via `writeError()` helper; no plain-text `http.Error`.
- **HTTP server** — `ReadTimeout=15s`, `WriteTimeout=30s`, `IdleTimeout=60s`; graceful shutdown via `signal.NotifyContext` + 10 s drain.
- **SQLite requires CGO** — `go-sqlite3` must be compiled with CGO enabled. The Dockerfile uses the full Go image for the build stage.
- **Schema migration** — `setupSchema` in `store.go` runs at startup; uses `ALTER TABLE` for additive migrations, safe to re-run.
- **Static serving** — in production the server serves `web/dist/` as static files; SPA fallback routes unknowns to `index.html`.
- **Bots** — Telegram and Discord bots are optional; each starts only when its credentials env vars are set. Both delegate command parsing to `internal/botcore.Commander`.

### Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `DB_PATH` | `tasks.db` | SQLite file path |
| `AUTH_SECRET` | (random) | HMAC secret — set for session persistence across restarts |
| `PORT` | `8080` | Listen port |
| `ALLOW_REGISTRATION` | `true` | Public signup |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | `admin@example.com` / `admin` | Bootstrap admin credentials |
| `CORS_ORIGIN` | `*` | Allowed CORS origin |
| `TELEGRAM_ENABLED` | `true` | Set to `false` to disable Telegram bot |
| `BOT_TOKEN` / `BOT_CHAT_ID` | — | Telegram bot credentials |
| `DISCORD_ENABLED` | `true` | Set to `false` to disable Discord bot |
| `DISCORD_TOKEN` / `DISCORD_CHANNEL_ID` / `DISCORD_GUILD_ID` | — | Discord bot credentials |

### Data model

`projects → tasks → task_comments`; `users ↔ projects` via `user_projects` join table. Task statuses: `new`, `in_progress`, `done`.
