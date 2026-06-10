#!/usr/bin/env bash
# Local dev loop: Go API via air (:8080) + Vite dev server (:3000).
# Vite proxies /api and /healthz to the Go server; open http://localhost:3000.
set -euo pipefail
cd "$(dirname "$0")/.."

if ! command -v air >/dev/null 2>&1; then
  echo "air not found — install with: go install github.com/air-verse/air@latest" >&2
  exit 1
fi


(cd frontend && bun install)

trap 'kill 0' EXIT

(cd frontend && exec bun run dev) &
air &
wait
