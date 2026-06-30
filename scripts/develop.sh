#!/bin/sh
# Container dev loop: Go API via air (:8080) + Vite dev server (:3000).
# Vite proxies /api and /healthz to the Go server; open http://localhost:3000.
set -e

cd /app

echo "==> Installing frontend dependencies..."
cd frontend
bun install
cd ..

echo "==> Starting Vite dev server (:3000)..."
cd frontend
bun run dev &
VITE_PID=$!
cd ..

# Give Vite a moment to start
sleep 2

# Bail out if Vite already died so we don't run a half-up dev loop.
if ! kill -0 "$VITE_PID" 2>/dev/null; then
  echo "==> Vite dev server failed to start" >&2
  exit 1
fi

echo "==> Starting Go backend with Air hot-reload (:8080)..."
air -c .air.toml

# If Air exits, clean up Vite
kill "$VITE_PID" 2>/dev/null || true
