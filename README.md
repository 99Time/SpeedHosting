# SpeedHosting

SpeedHosting is a clean foundation for a multi-user PUCK server hosting platform.

## Stack

- Backend: Go + Chi + SQLite
- Frontend: React + TypeScript + Vite
- Database: SQLite with bootstrap migration and seed data

## Project layout

```text
SpeedHosting/
  backend/
    cmd/speedhosting-api/        # API entrypoint
    internal/config/             # Environment and runtime config
    internal/httpserver/         # Router, handlers, middleware
    internal/models/             # API and domain models
    internal/services/           # Auth, admin, dashboard, plan, server
    internal/store/              # SQLite init, migration, seeds
  frontend/
    src/app/                     # App and routes
    src/components/              # Layout and reusable UI
    src/lib/                     # API client
    src/pages/                   # Landing, auth, dashboard pages
    src/styles/                  # Theme and application styling
```

## Initial database model

- `plans`: plan definitions and platform limits
- `users`: account identity and plan ownership
- `servers`: owned server definitions and desired configuration
- `server_admins`: future multi-admin support per server
- `server_runtime`: runtime state, player counts, and last runtime-action diagnostics

## Local run

Backend:

```bash
cd backend
go run ./cmd/speedhosting-api
```

Frontend:

```bash
cd frontend
npm install
npm run dev
```

The frontend runs on `http://localhost:5173` and proxies API calls to `http://localhost:8081`.

## Runtime configuration

For a Linux VPS with an existing PUCK runtime, configure these environment variables for the backend service:

- `SPEEDHOSTING_PUCK_CONFIG_DIR=/srv/puckserver`
- `SPEEDHOSTING_PUCK_TEMPLATE_CONFIG=/srv/puckserver/server1.json`
- `SPEEDHOSTING_PUCK_SERVICE_PREFIX=puck@`
- `SPEEDHOSTING_PUCK_SYSTEMCTL_PATH=/usr/bin/systemctl`
- `SPEEDHOSTING_PUCK_BASE_PORT=7777`
- `SPEEDHOSTING_PUCK_RESERVED_PORTS=7777-7786`

New hosted servers are created as real JSON files under `/srv/puckserver` using the `server_<sanitized_name>.json` pattern and controlled through the configured systemd template prefix.

Ports `7777` through `7786` are treated as reserved by default, so SpeedHosting will not assign new hosted servers to any of those ports.

The first registered account becomes the initial platform admin automatically. Admins can review the full fleet and change user plans from the built-in admin panel.
