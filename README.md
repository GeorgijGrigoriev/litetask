# LiteTask

LiteTask is a lightweight task board with projects, comments, and user roles.
It ships a Go backend and a React (Vite) frontend, packaged as a single app.

## Features
- Projects with per-project task boards
- Task details with comments
- Admin user management and project access
- Optional Telegram notifications (env-based)

## Requirements
- Go 1.25.1
- Node.js 20+
- Docker (optional)

## Local Development

Backend:
```bash
go run ./cmd/litetask
```

Frontend (dev server, proxies `/api` to the backend):
```bash
cd web
npm ci
npm run dev
```

## Build

Frontend build (outputs to `web/dist/`):
```bash
cd web
npm ci
npm run build
```

Backend build:
```bash
go build ./cmd/litetask
```

## Docker

Build image:
```bash
docker build -t litetask:latest .
```

Run:
```bash
docker run --rm -p 8080:8080 \
  -e AUTH_SECRET="change-me-32-bytes-min" \
  -e ALLOW_REGISTRATION=true \
  -v litetask-data:/data \
  litetask:latest
```

The app will be available at http://localhost:8080.

### docker-compose.yaml

Use the included `docker-compose.yaml`:
```bash
docker compose up --build
```

## Configuration

Key environment variables:
- `DB_PATH` (default: `/data/tasks.db`)
- `AUTH_SECRET` (required for persistent sessions; 32+ bytes or base64)
- `ALLOW_REGISTRATION` (`true`/`false`)
- `PORT` (default: `8080`)
- `BOT_TOKEN`, `BOT_CHAT_ID` (optional)
