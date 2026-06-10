# AGENTS.md

This file is the canonical guide for AI agents working in this repository.
Human contributors should start with [README.md](README.md).

## What this is

BandWidth: practice tracking for musicians and bands. The approved design
lives at `docs/superpowers/specs/2026-06-10-bandwidth-design.md` — read it
before adding features. Implementation plans live in
`docs/superpowers/plans/`.

## Build & development commands

All checks and builds run through Dagger. Do not run `go test`,
`golangci-lint`, `eslint`, `prettier`, `tsc`, or `vitest` directly on the
host — use `just` (or `dagger call ...`). The exceptions are the dev loop
(`just dev` runs air and vite on the host) and dependency management
(`go get`, `bun add`).

- `just dev` — hot-reload dev loop (Go API :8080, Vite :3000; open :3000)
- `just check` — lint-go, lint-js, typecheck, format-check, test-go,
  test-frontend, all in parallel in one Dagger session
- `just fmt` / `just format` — goimports / Prettier write mode
- `just build` — production container tarball at `tmp/bandwidth-image.tar`

## Architecture

- `cmd/bandwidth/` — Cobra entry point, Viper config (`BANDWIDTH_*` env
  vars), Echo server wiring, graceful shutdown
- `internal/handlers/` — HTTP handlers (healthz, SPA fallback)
- `internal/static/` — `go:embed` of the frontend build; locally only a
  placeholder, populated inside the Dagger release build
- `version/` — version string injected via ldflags
- `frontend/` — React SPA (Vite, React Router, Tailwind v4 + DaisyUI 5);
  Vite dev server proxies `/api` and `/healthz` to the Go server
- `scripts/develop.sh` — the dev loop `just dev` runs (bun install, then
  vite + air concurrently)
- `.github/` — GitHub Actions CI (`workflows/ci.yml` calls the same Dagger
  functions as `just`) and Renovate config
- `.dagger/` — CI pipeline; the justfile is a thin wrapper over it

## Style guides & documented deviations

This repo follows the conventions from the `style-guides` repo (local copy
at `/Users/john/code/git/style-guides/` on the primary dev machine).
Deviations:

- **Parallel `Check` Dagger function instead of one CI job per task**
  (same trade-off as ReadWillBe: faster, coarser GitHub status; errgroup
  reports only the first failing gate).
- **Frontend tooling configs live in `frontend/`** (`eslint.config.js`,
  `.prettierrc.json`, `tsconfig.json`), not the repo root, because the
  frontend is a self-contained Vite app with its own package.json.
- **`tsconfig.json` does not extend `tsconfig.base.json`** — browser-bundle
  settings differ; the strict-family flags are copied inline.
- **`.golangci.yml` adds one narrow exclusion** to the canonical config:
  revive's var-naming rule flags the `version` package for colliding with
  stdlib `go/version`; the package name is kept (it mirrors the
  maintainer's other apps) and the single finding is excluded by
  path + text.

## Testing

- Go: table-driven tests alongside source (`*_test.go`); handlers tested
  through `e.ServeHTTP` with `httptest`; SPA handler takes an `fs.FS` so
  tests use `fstest.MapFS`.
- Frontend: Vitest + Testing Library (jsdom); setup in
  `frontend/src/test/setup.ts`.

## Echo v5 notes

Echo v5 handlers/middleware use pointer contexts: `func(c *echo.Context)
error`. `c.Response()` returns `http.ResponseWriter`; use
`echo.ResolveResponseStatus(c.Response(), err)` to read a response status
in middleware. The HTTP server is stdlib `http.Server` with Echo as
handler (explicit graceful shutdown).
