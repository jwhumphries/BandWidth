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

echo "==> Starting Go backend with Air hot-reload (:8080)..."
air -c .air.toml

# If Air exits, clean up Vite
kill "$VITE_PID" 2>/dev/null || true
