#!/usr/bin/env bash
set -euo pipefail

echo "[run.sh] Starting service"

echo "[run.sh] Running DB migrations"
goose -dir ./db/migrations postgres "${DATABASE_URL}" up

echo "[run.sh] Starting Caddy"
caddy run --config /etc/caddy/Caddyfile --adapter caddyfile &

echo "[run.sh] Starting Go app"
export BACKEND_PORT="${BACKEND_PORT:-8080}"
export BACKEND_HOST="${BACKEND_HOST:-127.0.0.1}"
exec /app/bin/app
