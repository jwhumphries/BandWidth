# PWA & Deploy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make BandWidth an installable, auto-updating PWA and ship it to fly.io as a single always-on container, deployed by mirroring the sibling blog repo's Dagger→GHCR→Fly pipeline.

**Architecture:** The production container already exists — `internal/static/static.go` embeds the built frontend via `go:embed all:dist`, and the Dagger `Release` function builds a static CGO-free binary into a nonroot Alpine image on `:8080` with a `/healthz` endpoint. This plan adds (1) the PWA layer in the frontend — `vite-plugin-pwa` (Workbox) with `registerType: 'prompt'` (the design doc's `autoUpdate` intent realized as a reload toast — see Task 1), a precached app shell, **NetworkOnly for `/api`** (no stale data in v1), a web manifest, and an "update available → reload" toast wired to the service-worker lifecycle; and (2) the deploy layer — a Dagger `Publish` function that pushes the `Release` image to GHCR, a `publish.yml` GitHub Actions workflow (publish → `flyctl deploy`), and a `fly.toml` that references the GHCR image and runs exactly one always-on machine with a Fly volume for the SQLite file. Sessions are opaque DB tokens (`internal/auth/token.go`), so there is no signing secret to manage.

**Icons are deferred:** real maskable PNG icons (192/512) are pending artwork from the maintainer. The manifest and service worker land now with an empty `icons` array; the app is fully functional and the SW/precache/offline-shell work, but it is not yet *installable* until the icons land. Task 7 is the one-step follow-up to drop in the icons and the manifest entries.

**Tech Stack:** React 19 + TypeScript + Vite + `vite-plugin-pwa`/Workbox, Go 1.26 (`go:embed`), Dagger (Go module `bandwidth`), GitHub Actions, fly.io + flyctl, GHCR. All build/verify runs through `just` recipes; only dependency installs and the dev loop run on the host.

---

## File Structure

**Frontend (PWA)**
- `frontend/package.json` — add `vite-plugin-pwa` dev dependency.
- `frontend/vite.config.ts` — register `VitePWA(...)` with manifest + Workbox runtime caching.
- `frontend/index.html` — add `theme-color` meta + description.
- `frontend/src/components/UpdateToast.tsx` (new) — SW lifecycle toast.
- `frontend/src/components/UpdateToast.test.tsx` (new).
- `frontend/src/App.tsx` — mount `<UpdateToast />`.
- `frontend/src/vite-env.d.ts` (or `src/pwa.d.ts`) — types for the `virtual:pwa-register/react` module.

**Deploy**
- `.dagger/main.go` — add `Publish` (and a small `gitVersion` helper if not present).
- `Justfile` — add a `publish` recipe (optional local use).
- `fly.toml` (new, repo root).
- `.github/workflows/publish.yml` (new).
- `DEPLOY.md` (new) — the deploy runbook (volume, secrets, first deploy).
- `AGENTS.md`, `README.md` — document the PWA + deploy.

---

### Task 1: Add vite-plugin-pwa with manifest + NetworkOnly /api

**Files:**
- Modify: `frontend/package.json` (dependency), `frontend/vite.config.ts`, `frontend/index.html`
- Create: `frontend/src/pwa.d.ts`

- [ ] **Step 1: Add the dependency (host install is allowed for deps).**

Run on the host:
```bash
cd frontend && bun add -D vite-plugin-pwa && cd ..
```
Expected: `vite-plugin-pwa` appears under `devDependencies` in `frontend/package.json` and `bun.lock` updates.

- [ ] **Step 2: Configure VitePWA** in `frontend/vite.config.ts`. The current file is:

```ts
import {defineConfig} from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
});
```

Replace it with:

```ts
import {defineConfig} from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import {VitePWA} from 'vite-plugin-pwa';

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      // 'prompt' (not the design doc's literal 'autoUpdate') so the SW exposes
      // needRefresh and the reload toast (Task 2) actually fires — autoUpdate
      // silently swaps the SW and never triggers onNeedRefresh, leaving the
      // toast dead. This honors the design doc's intent (a reload toast).
      registerType: 'prompt',
      // Icons are deferred until artwork lands; the manifest is otherwise
      // complete. Add 192/512 maskable PNG entries here to make the app
      // installable (see Task 7).
      manifest: {
        name: 'BandWidth',
        short_name: 'BandWidth',
        description: 'Practice tracking for musicians and bands',
        theme_color: '#1d4ed8',
        background_color: '#ffffff',
        display: 'standalone',
        start_url: '/',
        icons: [],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,svg,woff2}'],
        // v1: never serve stale API data — the SPA always hits the network
        // for /api and shows its own loading/error states.
        runtimeCaching: [
          {
            urlPattern: ({url}) => url.pathname.startsWith('/api'),
            handler: 'NetworkOnly',
          },
        ],
        // SPA deep links fall back to index.html (matches RegisterSPA on the
        // server), but never for /api requests.
        navigateFallback: '/index.html',
        navigateFallbackDenylist: [/^\/api/],
      },
    }),
  ],
});
```

- [ ] **Step 3: Add PWA virtual-module types** — create `frontend/src/pwa.d.ts`:

```ts
/// <reference types="vite-plugin-pwa/react" />
/// <reference types="vite-plugin-pwa/info" />
```

- [ ] **Step 4: Add manifest meta to `frontend/index.html`.** Replace the `<head>` block's content so it includes a theme color and description (vite-plugin-pwa injects the manifest link automatically):

```html
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta name="theme-color" content="#1d4ed8" />
    <meta
      name="description"
      content="Practice tracking for musicians and bands"
    />
    <title>BandWidth</title>
  </head>
```

- [ ] **Step 5: Verify the production build emits the service worker + manifest.**

Run: `just build-frontend` (Dagger). Then confirm the emitted artifacts exist:
```bash
ls frontend/dist/manifest.webmanifest frontend/dist/sw.js
```
Expected: both files exist. Then run `just typecheck && just lint-js && just format-check && just test-frontend` — all green (run `just format` first if format-check fails). The existing tests must still pass; `virtual:pwa-register` is only imported by the component added in Task 2, so nothing breaks here.

- [ ] **Step 6: Commit**

```bash
git add frontend
git commit -m "feat: add vite-plugin-pwa manifest and service worker (NetworkOnly /api)"
```

---

### Task 2: Update-available toast wired to the SW lifecycle

**Files:**
- Create: `frontend/src/components/UpdateToast.tsx`, `frontend/src/components/UpdateToast.test.tsx`
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Write the failing test** — `frontend/src/components/UpdateToast.test.tsx`. Mock the virtual register hook so the test controls `needRefresh`:

```tsx
import {screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {render} from '@testing-library/react';

const updateServiceWorker = vi.fn();
let needRefresh = true;

vi.mock('virtual:pwa-register/react', () => ({
  useRegisterSW: () => ({
    needRefresh: [needRefresh, vi.fn()],
    offlineReady: [false, vi.fn()],
    updateServiceWorker,
  }),
}));

// Import AFTER the mock is registered.
import UpdateToast from './UpdateToast';

describe('UpdateToast', () => {
  beforeEach(() => {
    updateServiceWorker.mockClear();
    needRefresh = true;
  });

  it('shows a reload prompt when an update is ready and reloads on click', async () => {
    render(<UpdateToast />);
    expect(screen.getByText(/new version available/i)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: /reload/i}));
    expect(updateServiceWorker).toHaveBeenCalledWith(true);
  });

  it('renders nothing when no update is pending', () => {
    needRefresh = false;
    const {container} = render(<UpdateToast />);
    expect(container).toBeEmptyDOMElement();
  });
});
```

Run: `just test-frontend`. Expected: FAIL — cannot resolve `./UpdateToast`.

> **Note (virtual module under Vitest):** `frontend/vitest.config.ts` is separate from `vite.config.ts`, so the VitePWA plugin is **not** loaded in tests and `virtual:pwa-register/react` has no resolver. The `vi.mock('virtual:pwa-register/react', …)` factory above should intercept it. If Vitest instead errors with `Failed to resolve import "virtual:pwa-register/react"`, add a stub alias to `frontend/vitest.config.ts` so the bare import resolves before the mock applies — create `frontend/src/test/pwa-register-stub.ts` exporting `export const useRegisterSW = () => ({needRefresh: [false, () => {}], offlineReady: [false, () => {}], updateServiceWorker: () => {}});` and map it under `test: { alias: {'virtual:pwa-register/react': new URL('./src/test/pwa-register-stub.ts', import.meta.url).pathname} }`. Report whichever path was needed.

- [ ] **Step 2: Write `frontend/src/components/UpdateToast.tsx`**

```tsx
import {useRegisterSW} from 'virtual:pwa-register/react';

// UpdateToast surfaces the service-worker "new version ready" event as a
// small toast. With registerType: 'prompt', needRefresh becomes true when a
// new SW is waiting; clicking Reload activates it and reloads the page.
export default function UpdateToast() {
  const {
    needRefresh: [needRefresh],
    updateServiceWorker,
  } = useRegisterSW();

  if (!needRefresh) {
    return null;
  }

  return (
    <div className="toast toast-end z-50">
      <div className="alert alert-info">
        <span>New version available.</span>
        <button
          className="btn btn-sm"
          onClick={() => void updateServiceWorker(true)}
        >
          Reload
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Mount it in `frontend/src/App.tsx`.** The current `App` returns `<Routes>…</Routes>` directly with no wrapper element, so wrap the return in a fragment and render `<UpdateToast />` as a sibling so it shows on every route:

```tsx
import UpdateToast from './components/UpdateToast';
// ...
export default function App() {
  return (
    <>
      <UpdateToast />
      <Routes>
        {/* …existing routes, unchanged… */}
      </Routes>
    </>
  );
}
```

- [ ] **Step 4: Verify.** `just test-frontend` (the new test passes; `virtual:pwa-register/react` resolves at build via the plugin and is mocked in tests), `just typecheck`, `just lint-js`, `just format-check` — all green (run `just format` if needed).

- [ ] **Step 5: Commit**

```bash
git add frontend
git commit -m "feat: service-worker update toast"
```

---

### Task 3: Dagger Publish function (image → GHCR)

**Files:**
- Modify: `.dagger/main.go`
- Modify: `Justfile`

- [ ] **Step 1: Add `Publish` to `.dagger/main.go`.** It reuses the existing `Release` container, tags it `:version` and `:latest`, and pushes to the registry with optional auth — mirroring the blog repo's `Publish`. Add these imports if missing (`context`, `fmt` — `context` is already used by `Check`; confirm `fmt` is imported). Append after `Release`:

```go
// Publish builds the production container and pushes it to a registry as
// both :version and :latest, returning the published refs. Mirrors the
// release image; auth is applied when credentials are supplied.
func (m *Bandwidth) Publish(
	ctx context.Context,
	// +ignore=["**/node_modules", "frontend/dist", "tmp", "bin", "data", ".git"]
	source *dagger.Directory,
	// Container registry address (e.g. ghcr.io/jwhumphries/bandwidth)
	// +optional
	// +default="ghcr.io/jwhumphries/bandwidth"
	registry string,
	// +optional
	// +default="dev"
	version string,
	// Registry username
	// +optional
	registryUser string,
	// Registry password (as a secret)
	// +optional
	registryPassword *dagger.Secret,
) (string, error) {
	if registry == "" {
		registry = "ghcr.io/jwhumphries/bandwidth"
	}
	container := m.Release(source, version)
	if registryUser != "" && registryPassword != nil {
		container = container.WithRegistryAuth(registry, registryUser, registryPassword)
	}
	versionRef, err := container.Publish(ctx, fmt.Sprintf("%s:%s", registry, version))
	if err != nil {
		return "", fmt.Errorf("publish version tag: %w", err)
	}
	if _, err := container.Publish(ctx, fmt.Sprintf("%s:latest", registry)); err != nil {
		return "", fmt.Errorf("publish latest tag: %w", err)
	}
	return versionRef, nil
}
```

(Confirm the receiver type name — the module type is `Bandwidth` per `Release`/`Check`. Match it. If `fmt` or `context` is not yet imported in `main.go`, add it.)

- [ ] **Step 2: Add a `publish` recipe to the `Justfile`** (for local/manual publishing; CI calls Dagger directly). Match the existing recipe style (see `build`):

```make
# Build and push the production image to a registry (CI uses GH credentials)
publish registry="ghcr.io/jwhumphries/bandwidth" version=`git rev-parse --short HEAD`:
    dagger call publish --source . --registry {{registry}} --version {{version}}
```

- [ ] **Step 3: Verify the Dagger module still compiles.** The module compiles whenever a function is invoked. Run:
```bash
just check
```
Expected: `all checks passed` (this invokes the module, proving `Publish` compiles). Do NOT run `just publish` here — it would require registry credentials and push an image.

- [ ] **Step 4: Commit**

```bash
git add .dagger/main.go Justfile
git commit -m "feat: dagger publish function for the production image"
```

---

### Task 4: fly.toml + publish/deploy workflow

**Files:**
- Create: `fly.toml`, `.github/workflows/publish.yml`

- [ ] **Step 1: Create `fly.toml`** at the repo root. Single always-on machine (SQLite can't multi-attach), a Fly volume mounted at `/data`, image pulled from GHCR, env wired to `BANDWIDTH_*`, health check on `/healthz`:

```toml
# fly.toml — BandWidth (single always-on machine; SQLite on a Fly volume)
# See https://fly.io/docs/reference/configuration/

app = "bandwidth"
primary_region = "ord"

[build]
  image = "ghcr.io/jwhumphries/bandwidth:latest"

[env]
  BANDWIDTH_PORT = ":8080"
  BANDWIDTH_DB_PATH = "/data/bandwidth.db"
  BANDWIDTH_SECURE_COOKIES = "true"
  BANDWIDTH_LOG_LEVEL = "info"
  # BANDWIDTH_BASE_URL is set per-app below; update it to the real hostname.
  BANDWIDTH_BASE_URL = "https://bandwidth.fly.dev"

[[mounts]]
  source = "bandwidth_data"
  destination = "/data"

[http_service]
  internal_port = 8080
  force_https = true
  # SQLite is single-writer on one volume: exactly one always-on machine.
  auto_stop_machines = false
  auto_start_machines = false
  min_machines_running = 1
  processes = ["app"]

  [[http_service.checks]]
    interval = "15s"
    timeout = "2s"
    grace_period = "5s"
    method = "GET"
    path = "/healthz"

[[vm]]
  memory = "256mb"
  cpu_kind = "shared"
  cpus = 1
```

(The `app` name and `primary_region` are placeholders the maintainer adjusts; `fly launch` will also prompt. `BANDWIDTH_BASE_URL` must match the real hostname for absolute links/secure cookies.)

- [ ] **Step 2: Create `.github/workflows/publish.yml`** — mirror the blog's pipeline: build+publish to GHCR with Dagger, then deploy to Fly. (Pinned action SHAs match the blog repo's.)

```yaml
name: Build and Publish Container

on:
  push:
    branches:
      - main
    tags:
      - 'v*'
  workflow_dispatch:

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}
  DAGGER_NO_NAG: "1"

jobs:
  build-and-publish:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - name: Checkout
        uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4
        with:
          fetch-depth: 0

      - name: Install Dagger
        uses: dagger/dagger-for-github@b81317a976cb7f7125469707321849737cd1b3bc # v7
        with:
          version: "latest"

      - name: Log in to GHCR
        uses: docker/login-action@c94ce9fb468520275223c153574b00df6fe4bcc9 # v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and publish container
        run: |
          dagger call publish \
            --source . \
            --registry "${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}" \
            --version "${{ github.sha }}" \
            --registry-user "${{ github.actor }}" \
            --registry-password env:GITHUB_TOKEN
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

  deploy-to-fly:
    runs-on: ubuntu-latest
    needs: build-and-publish
    if: github.ref == 'refs/heads/main'
    steps:
      - name: Checkout
        uses: actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4

      - name: Setup Fly CLI
        uses: superfly/flyctl-actions/setup-flyctl@master

      - name: Deploy to Fly.io
        run: flyctl deploy --remote-only
        env:
          FLY_API_TOKEN: ${{ secrets.FLY_API_TOKEN }}
```

(Note: `IMAGE_NAME` is `${{ github.repository }}` = `jwhumphries/BandWidth`; GHCR lowercases the path on push. If the mixed-case repo name causes a push error, hardcode `ghcr.io/jwhumphries/bandwidth` in both `fly.toml` and the `--registry` arg. Flag this in your report.)

- [ ] **Step 3: Verify the config is well-formed.** These files aren't covered by `just check`. Confirm TOML/YAML parse and the workflow references the right Dagger args. Run:
```bash
git add fly.toml .github/workflows/publish.yml
just check
```
Expected: `all checks passed` (unchanged; config files don't affect it). Visually confirm `fly.toml`'s image ref and `publish.yml`'s `dagger call publish` args match Task 3's function signature.

- [ ] **Step 4: Commit**

```bash
git commit -m "feat: fly.toml and publish/deploy workflow"
```

---

### Task 5: Production image smoke test + deploy runbook

**Files:**
- Create: `DEPLOY.md`

- [ ] **Step 1: Smoke-test the production image locally.** Build the release image tarball, load it into Docker, run it, and confirm it serves both the health endpoint and the SPA shell. (Docker is the host runtime here; the *image* was built by Dagger.)

```bash
just build                                   # dagger release → tmp/bandwidth-image.tar
docker load -i tmp/bandwidth-image.tar        # prints the loaded image ref
# Run with an ephemeral in-container db path:
docker run -d --name bw-smoke -p 8080:8080 -e BANDWIDTH_DB_PATH=/tmp/bw.db <loaded-image-ref>
sleep 2
curl -fsS http://localhost:8080/healthz       # expect 200
curl -fsS http://localhost:8080/ | grep -q '<div id="root">' && echo "SPA OK"
docker rm -f bw-smoke
```
Expected: `/healthz` returns 200 and the root serves the SPA `index.html`. If Docker isn't available in the environment, report that and verify instead that `just build` produces the tarball and that `RegisterSPA` + `Healthz` are wired in `cmd/bandwidth/server.go` (they are).

- [ ] **Step 2: Write `DEPLOY.md`** — the runbook for the maintainer's one-time setup and ongoing deploys:

```markdown
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
```

- [ ] **Step 3: Commit**

```bash
git add DEPLOY.md
git commit -m "docs: production image smoke test notes and deploy runbook"
```

---

### Task 6: Docs + final verification

**Files:**
- Modify: `AGENTS.md`, `README.md`

- [ ] **Step 1: Update `AGENTS.md`.** In the architecture/build section, note the PWA layer (`vite-plugin-pwa`, `registerType: 'prompt'` with the `UpdateToast` reload toast, NetworkOnly `/api`) and the deploy pipeline (Dagger `Publish` → GHCR → `fly.toml`/`publish.yml`, single always-on machine + Fly volume; see `DEPLOY.md`). Verify each claim against the code.

- [ ] **Step 2: Update `README.md`.** Change the roadmap/stack lines so the PWA and deploy are now implemented (icons pending), e.g.: "Installable PWA (service worker with auto-update and NetworkOnly API; app icons pending artwork) and single-container fly.io deploy (Dagger→GHCR→Fly) are implemented." Adapt to the real surrounding text; report the change.

- [ ] **Step 3: Final verification.** `just check` → "all checks passed". Confirm only the two doc files are dirty.

- [ ] **Step 4: Commit**

```bash
git add AGENTS.md README.md
git commit -m "docs: document the PWA and fly.io deploy"
```

---

### Task 7 (DEFERRED — do when icon artwork lands): wire PWA icons

**Do not execute until the maintainer provides icon artwork.** This is the only step between here and an *installable* PWA.

**Files:**
- Add: `frontend/public/icon-192.png`, `frontend/public/icon-512.png` (maskable), and `frontend/public/apple-touch-icon.png` (180×180, optional).
- Modify: `frontend/vite.config.ts` (manifest `icons`), `frontend/index.html` (apple-touch-icon link).

- [ ] **Step 1:** Drop the provided 192×192 and 512×512 maskable PNGs into `frontend/public/`.

- [ ] **Step 2:** Fill in the manifest `icons` array in `vite.config.ts`:

```ts
        icons: [
          {src: '/icon-192.png', sizes: '192x192', type: 'image/png', purpose: 'any maskable'},
          {src: '/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'any maskable'},
        ],
```

Add `png` to the Workbox `globPatterns` so the icons precache:
`globPatterns: ['**/*.{js,css,html,svg,png,woff2}']`.

- [ ] **Step 3:** Add `<link rel="apple-touch-icon" href="/apple-touch-icon.png" />` to `index.html` `<head>` (if an apple icon was provided).

- [ ] **Step 4:** `just build-frontend` then confirm `frontend/dist/manifest.webmanifest` lists the icons and they are precached. Run the four frontend checks. Optionally verify installability in a browser (Lighthouse PWA / "Install app" affordance appears).

- [ ] **Step 5:** Commit `feat: add PWA app icons (installable)`.

---

## Done criteria

- `just check` green (all six gates).
- `just build-frontend` emits `dist/sw.js` + `dist/manifest.webmanifest`; the SW precaches the app shell and uses NetworkOnly for `/api`.
- The update toast appears when a new SW is ready and reloads to the new version on click.
- The production image (from `just build`) serves `/healthz` (200) and the SPA shell, reads its DB path from `BANDWIDTH_DB_PATH`, and runs as a nonroot user on `:8080`.
- `fly.toml` pins the GHCR image, mounts the `/data` volume, and runs exactly one always-on machine with a `/healthz` check; `publish.yml` builds+publishes via Dagger and deploys via flyctl; `DEPLOY.md` documents the one-time setup.
- Deferred: drop in icon artwork (Task 7) to make the PWA installable.
- This completes the v1 roadmap from the design doc.
