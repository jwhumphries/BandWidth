# BandWidth

Practice tracking for musicians and bands. Track songs through
Not Learned → Learning → Learned → Nailed!, log practice days, organize
songs into folders, and share songs and statuses with your band.

Design: [docs/superpowers/specs/2026-06-10-bandwidth-design.md](docs/superpowers/specs/2026-06-10-bandwidth-design.md)

## Stack

Go + Echo + SQLite (GORM, CGO-free) backend; React 19 + TypeScript + Vite +
Tailwind CSS/DaisyUI frontend with TanStack Query. Accounts with TOTP 2FA
and optional SMTP password reset. Personal song tracking (status, notes, links,
practice days, folders) is implemented. Bands with roles, invites, and shared
songs (band metadata layer, personal-view interleaving, and a conversion engine
that preserves members' work on leave/delete) are implemented. Planned per the
design doc: band folders, installable PWA, single container on fly.io.

## Development

Requires `go`, `bun`, `just`, `dagger`, and `air`
(`go install github.com/air-verse/air@latest`).

```bash
just dev     # hot-reload dev loop: Go API :8080 + Vite :3000
just check   # all lints, type checks, and tests (in Dagger)
just build   # production container image tarball
```

`just --list` shows all recipes.
