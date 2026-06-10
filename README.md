# BandWidth

Practice tracking for musicians and bands. Track songs through
Not Learned → Learning → Learned → Nailed!, log practice days, organize
songs into folders, and share songs and statuses with your band.

Design: [docs/superpowers/specs/2026-06-10-bandwidth-design.md](docs/superpowers/specs/2026-06-10-bandwidth-design.md)

## Stack

Go + Echo backend with React 19 + TypeScript + Vite + Tailwind CSS/DaisyUI
frontend. Planned per the design doc: SQLite (GORM) persistence, installable
PWA, single container on fly.io.

## Development

Requires `go`, `bun`, `just`, `dagger`, and `air`
(`go install github.com/air-verse/air@latest`).

```bash
just dev     # hot-reload dev loop: Go API :8080 + Vite :3000
just check   # all lints, type checks, and tests (in Dagger)
just build   # production container image tarball
```

`just --list` shows all recipes.
