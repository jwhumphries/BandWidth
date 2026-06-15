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
- `internal/handlers/` — HTTP handlers (healthz, SPA fallback, auth,
  account, 2FA, password reset, songs, practice, resources, folders, bands, members, invites, band songs, band rehearsals/resources) sharing one `API` dependency struct
- `internal/static/` — `go:embed` of the frontend build; locally only a
  placeholder, populated inside the Dagger release build
- `internal/model/` — persisted domain types (User, Session, BackupCode,
  PasswordReset, Song, SongAnnotation, Resource, PracticeEvent, Folder, FolderEntry, Band, BandMember, BandInvite)
- `internal/repository/` — GORM/SQLite persistence (`Repo` struct; CGO-free
  driver `ncruces/go-sqlite3` via gormlite, WAL mode, AutoMigrate at startup);
  `bandsongs.go` houses the conversion engine; `subject.go` defines the `subj`
  value backing user- and band-keyed metadata methods
- `internal/auth/` — argon2id hashing, random tokens, TOTP, backup codes
- `internal/middleware/` — `RequireAuth` session middleware (import-aliased
  `appmw` where Echo's middleware package is also imported)
- `internal/mail/` — provider-agnostic SMTP mailer; disabled (and password
  reset hidden, endpoints 404) when `BANDWIDTH_SMTP_HOST`/`FROM` are unset
- `version/` — version string injected via ldflags
- `frontend/` — React SPA (Vite, React Router, Tailwind v4 + DaisyUI 5);
  Vite dev server proxies `/api` and `/healthz` to the Go server
- `scripts/develop.sh` — the dev loop `just dev` runs (bun install, then
  vite + air concurrently)
- `.github/` — GitHub Actions CI (`workflows/ci.yml` calls the same Dagger
  functions as `just`) and Renovate config
- `.dagger/` — CI pipeline; the justfile is a thin wrapper over it

## Configuration

All config is `BANDWIDTH_*` env vars via Viper (defaults in
`cmd/bandwidth/main.go`): `PORT` (:8080), `LOG_LEVEL` (info), `DB_PATH`
(data/bandwidth.db), `SECURE_COOKIES` (false; set true behind TLS),
`BASE_URL` (http://localhost:3000; used in password-reset links),
`SMTP_HOST`, `SMTP_PORT` (587), `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`.

Sessions are opaque 256-bit tokens stored hashed (SHA-256) in the DB and
carried in an HttpOnly Lax cookie — there is no cookie-signing secret.
CSRF uses Echo v5's fetch-metadata-aware middleware (`Sec-Fetch-Site`);
tests must set `Sec-Fetch-Site: same-origin` on mutating requests.
Login/signup/reset endpoints are rate limited (1 req/s, burst 5, per IP).

## Domain model

Songs are identity-only (title/artist + owner); ALL metadata — status
(`not_learned|learning|learned|nailed`), notes, resources, practice events —
lives in subject-keyed rows (subject = a user now; band columns exist but
are unwritten until the bands plan). A missing annotation row reads as
not_learned/empty and is created lazily on first edit. Practice events are
unique per (song, subject, date) with YYYY-MM-DD string dates; the client
sends its local date. Folders are playlist-style (one song may be in many
folders) with integer positions reindexed on reorder; deleting a folder
never deletes songs.

Bands have Admin/Editor/Viewer roles; the creator is a permanent Admin
(cannot be demoted, removed, or leave — they delete the band). New members
default to Editor. `MemberRole(bandID, userID)` is the authorization
primitive; non-members receive 404s for band resources. Direct invites are
single-use (14-day expiry); share links are multi-use until revoked (7-day
expiry); invite tokens are stored hashed.

Band songs are `owner_band_id`-owned and carry a full band metadata layer
(status, notes, resources, and a rehearsal log = band-keyed practice
events), edited only from the band view by Editors/Admins. Metadata
operations are written once against an internal `subj` value (a user XOR a
band) and exposed as user- and band-keyed methods. A band song appears in
every member's personal library with the member's OWN editable layer
(personal status/notes/resources/practice) plus a read-only band section;
its title/artist are band-owned and it cannot be deleted from the personal
view. When a member loses access to a band song (they leave or are removed,
the band deletes the song, or the band is deleted), any personal rows they
have on it are re-pointed onto a freshly created personal-copy song (the
band layer is not copied); untouched band songs simply leave their library.
All conversion runs in the same transaction as the removal. Band folders
arrive in the bands-folders plan.

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
- **`.golangci.yml` adds narrow exclusions** to the canonical config:
  revive's var-naming rule flags packages whose names collide with stdlib
  packages; `version` (go/version) and `mail` (net/mail) are intentional
  names mirroring the maintainer's other apps, excluded by path + text.

## Testing

- Go: table-driven tests alongside source (`*_test.go`); handlers tested
  through `e.ServeHTTP` with `httptest`; SPA handler takes an `fs.FS` so
  tests use `fstest.MapFS`.
- Repository tests run against in-memory SQLite (`repository.Open(":memory:")`,
  fresh DB per test). Handler tests register routes directly on a bare
  `echo.New()` (no CSRF) via helpers in `internal/handlers/auth_test.go`.
- Frontend: Vitest + Testing Library (jsdom); setup in
  `frontend/src/test/setup.ts`.

## Echo v5 notes

Echo v5 handlers/middleware use pointer contexts: `func(c *echo.Context)
error`. `c.Response()` returns `http.ResponseWriter`; use
`echo.ResolveResponseStatus(c.Response(), err)` to read a response status
in middleware. The HTTP server is stdlib `http.Server` with Echo as
handler (explicit graceful shutdown).
