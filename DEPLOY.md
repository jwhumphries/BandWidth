# Deploying BandWidth

BandWidth runs as a single always-on machine on fly.io with the SQLite
database on a Fly volume. The container image is built by Dagger and pushed
to GHCR; Fly pulls that image (it does not build anything).

## One-time setup

1. **Create the app** (matches `app` in `fly.toml`):
   ```bash
   fly apps create bandwidth
   ```
2. **Create the volume** in the same region as `primary_region`:
   ```bash
   fly volumes create bandwidth_data --region ord --size 1
   ```
3. **Set runtime config.** Non-secret values live in `fly.toml [env]`. Set
   the hostname-dependent and any SMTP values as needed:
   ```bash
   fly secrets set BANDWIDTH_BASE_URL=https://bandwidth.fly.dev
   # Optional email (invites / password reset); unset = those emails are skipped:
   fly secrets set BANDWIDTH_SMTP_HOST=... BANDWIDTH_SMTP_PORT=587 \
     BANDWIDTH_SMTP_USER=... BANDWIDTH_SMTP_PASS=... BANDWIDTH_SMTP_FROM=...
   ```
   There is no session secret — sessions are opaque tokens stored in SQLite.
4. **Add a `FLY_API_TOKEN` repo secret** for the GitHub Actions deploy job:
   ```bash
   fly tokens create deploy   # paste into the repo's Actions secrets
   ```
5. **Make the GHCR package readable by Fly** (public, or grant Fly pull
   access) so `fly deploy` can pull `ghcr.io/jwhumphries/bandwidth:latest`.

## Deploys

Pushing to `main` runs `.github/workflows/publish.yml`: Dagger builds and
pushes the image to GHCR, then `flyctl deploy --remote-only` rolls it out.
Manual deploy: `fly deploy --remote-only` (uses the image in `fly.toml`).

## Notes

- One machine only — SQLite cannot be attached to two machines. Do not scale
  `min_machines_running` above 1 or enable auto start/stop.
- The database file lives at `/data/bandwidth.db` on the volume and survives
  deploys. Back it up by copying the file off the volume (optional:
  Litestream later).
