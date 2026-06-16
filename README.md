# BandWidth

Practice tracking for musicians and bands. Track songs through
Not Learned → Learning → Learned → Nailed!, log practice days, organize
songs into folders, and share songs and statuses with your band.

Design: [docs/superpowers/specs/2026-06-10-bandwidth-design.md](docs/superpowers/specs/2026-06-10-bandwidth-design.md)

## Stack

Go + Echo + SQLite (GORM, CGO-free) backend; React 19 + TypeScript + Vite +
Tailwind CSS/DaisyUI frontend with TanStack Query. Accounts with TOTP 2FA
and optional SMTP password reset. Personal song tracking (status, notes, links,
practice days, folders) is implemented. Bands with roles, invites, shared
songs, and band folders (with personal-folder cross-inclusion and a conversion
engine that preserves members' work and folder placements on leave/delete) are
implemented. Installable PWA (service worker with an update-reload toast and
NetworkOnly API; app icons pending artwork) and single-container fly.io deploy
(Dagger→GHCR→Fly, one always-on machine with a volume for SQLite) are
implemented.

## Development

Requires `go`, `bun`, `just`, `dagger`, and `air`
(`go install github.com/air-verse/air@latest`).

```bash
just dev     # hot-reload dev loop: Go API :8080 + Vite :3000
just check   # all lints, type checks, and tests (in Dagger)
just build   # production container image tarball
```

`just --list` shows all recipes.
