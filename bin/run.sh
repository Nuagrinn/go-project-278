#!/usr/bin/env bash
set -euo pipefail

echo "[run.sh] Starting service"

echo "[run.sh] Running DB migrations"
goose -dir ./db/migrations postgres "${DATABASE_URL}" up

echo "[run.sh] Starting Go app"
export BACKEND_PORT="${BACKEND_PORT:-8081}"
export BACKEND_HOST="${BACKEND_HOST:-127.0.0.1}"
/app/bin/app &
app_pid="$!"

trap 'kill "$app_pid"' EXIT

echo "[run.sh] Starting Caddy"
exec caddy run --config /app/Caddyfile --adapter caddyfile
