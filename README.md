# LiteTask

LiteTask is a lightweight self-hosted task board with projects, comments, and user roles.
It ships a Go backend and a React (Vite + Ant Design) frontend packaged as a single Docker image.

## Features

- Projects with per-project Kanban-style task boards
- Task details with comments
- Role-based access: `admin` (full access), `user` (assigned projects only), `blocked`
- Admin panel: user management, role changes, project assignment
- Optional Telegram bot (long-polling)
- Optional Discord bot (slash commands)

## Requirements

- Go 1.25+ with CGO enabled (`go-sqlite3` requires CGO)
- Node.js 20+
- Docker (optional)

## Local Development

Start the backend (listens on `:8080`):
```bash
go run ./cmd/litetask
```

Start the frontend dev server (listens on `:5173`, proxies `/api` to `:8080`):
```bash
cd web
npm ci
npm run dev
```

The default admin account is `admin@example.com` / `admin` (override via env vars).

## Build

Frontend (outputs to `web/dist/`):
```bash
cd web
npm ci
npm run build
```

Backend (embeds `web/dist/` as static files):
```bash
go build ./cmd/litetask
```

## Docker

Build and run with the included `docker-compose.yaml`:
```bash
docker compose up --build
```

Or build and run manually:
```bash
docker build -t litetask:latest .

docker run --rm -p 8080:8080 \
  -e AUTH_SECRET="change-me-at-least-32-bytes" \
  -e ALLOW_REGISTRATION=true \
  -v litetask-data:/data \
  litetask:latest
```

The app will be available at http://localhost:8080.

## Configuration

All configuration is via environment variables.

| Variable | Default | Description |
|---|---|---|
| `DB_PATH` | `tasks.db` | SQLite database file path |
| `AUTH_SECRET` | (random) | HMAC secret for session cookies — set to persist sessions across restarts (32+ bytes or base64) |
| `PORT` | `8080` | Listen address (e.g. `:8080` or `0.0.0.0:8080`) |
| `ALLOW_REGISTRATION` | `true` | Allow public self-registration |
| `ADMIN_EMAIL` | `admin@example.com` | Bootstrap admin email |
| `ADMIN_PASSWORD` | `admin` | Bootstrap admin password |
| `CORS_ORIGIN` | `*` | Allowed CORS origin |
| `TELEGRAM_ENABLED` | `true` | Set to `false` to disable the Telegram bot |
| `BOT_TOKEN` | — | Telegram bot token |
| `BOT_CHAT_ID` | — | Telegram chat ID for notifications |
| `DISCORD_ENABLED` | `true` | Set to `false` to disable the Discord bot |
| `DISCORD_TOKEN` | — | Discord bot token |
| `DISCORD_CHANNEL_ID` | — | Discord channel ID for notifications |
| `DISCORD_GUILD_ID` | — | Discord guild (server) ID for slash command registration |

## Testing

```bash
go test ./...           # all tests
go test -race ./...     # with race detector (used in CI)
golangci-lint run       # lint
```

## Contributing

Contributions are welcome — PRs, bug reports, ideas, all good.

A few asks:
- Follow the existing patterns in the repo (code style, error handling, test conventions).
- If you're using an AI coding agent, `CLAUDE.md` in the root has project-specific guidance for it — point your agent there before starting.

No CLA, no formal process. Just open a PR.

## Architecture

```
cmd/litetask/        Entry point — wires everything together
internal/httpapi/    HTTP API server, session auth, CORS
internal/store/      SQLite data layer; store.Storer interface
internal/botcore/    Shared bot command handler (Commander)
internal/tgbot/      Telegram bot
internal/discordbot/ Discord bot
internal/config/     Env-var helper
web/src/             React SPA (Vite + Ant Design)
```

Data model: `projects → tasks → task_comments`; `users ↔ projects` via `user_projects` join table.
Task statuses: `new`, `in_progress`, `done`.
