# BandWidth Skeleton + CI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A working application skeleton: Go/Echo API with `/healthz`, embedded React/Vite/Tailwind/DaisyUI frontend with SPA fallback, hot-reload dev loop, and a full just + Dagger CI pipeline mirrored in GitHub Actions.

**Architecture:** Single Go binary embeds the built frontend via `go:embed` and serves it with an SPA fallback; in development, Vite (`:3000`) proxies `/api` and `/healthz` to the Go server (`:8080`) run by air. All lint/test/build runs inside Dagger containers; the justfile is a thin wrapper. This is Plan 1 of 5 from the spec (`docs/superpowers/specs/2026-06-10-bandwidth-design.md`); no database, auth, or domain features yet.

**Tech Stack:** Go 1.26, Echo v5, Cobra + Viper, slog; React 19 + TypeScript, Vite 7, React Router 7, Tailwind CSS v4 + DaisyUI 5, Vitest + Testing Library, bun; just + Dagger; GitHub Actions + Renovate.

---

## Conventions for the executor

- Repo root: `/Users/john/code/git/BandWidth`. All paths below are relative to it.
- The canonical style-guide configs live at `/Users/john/code/git/style-guides/` (this machine only).
- Work happens on `main` (solo project; spec was committed to `main`).
- Bootstrap tasks (1–7) run `go`/`bun` on the host because the Dagger pipeline doesn't exist yet. From Task 8 onward, all checks run via `just` (Dagger).
- Module path is `github.com/jwhumphries/bandwidth`.

## File structure being built

```
justfile, dagger.json, .golangci.yml, .gitignore, .air.toml
README.md, CLAUDE.md, AGENTS.md
.dagger/                      # Dagger module (main.go + generated code)
.github/workflows/ci.yml, .github/renovate.json
cmd/bandwidth/main.go         # Cobra root command, Viper config
cmd/bandwidth/server.go       # http.Server, Echo wiring, request logger
cmd/bandwidth/server_test.go
internal/handlers/health.go   # GET /healthz
internal/handlers/health_test.go
internal/handlers/spa.go      # SPA fallback file server over fs.FS
internal/handlers/spa_test.go
internal/static/static.go     # go:embed of frontend build
internal/static/dist/.gitkeep # placeholder so the embed compiles
version/version.go            # ldflags-injected version
scripts/develop.sh            # air + vite dev loop
frontend/
  package.json, bun.lock, index.html
  vite.config.ts, vitest.config.ts, tsconfig.json
  eslint.config.js, .prettierrc.json, .prettierignore
  src/main.tsx, src/App.tsx, src/index.css
  src/pages/HomePage.tsx, src/pages/HomePage.test.tsx
  src/test/setup.ts
```

---

### Task 0: Verify host prerequisites

- [ ] **Step 1: Check required tools**

Run:
```bash
go version && bun --version && just --version && dagger version && git status --porcelain
```
Expected: Go 1.26+, bun 1.x, just, dagger ≥ 0.19, and an empty `git status` (clean tree). If `air` is missing (checked later by the dev script): `go install github.com/air-verse/air@latest`.

---

### Task 1: Repo scaffolding and lint config

**Files:**
- Create: `.gitignore`
- Create: `.golangci.yml` (copied from style-guides)

- [ ] **Step 1: Write `.gitignore`**

```gitignore
# Build artifacts
tmp/
bin/
frontend/dist/
frontend/node_modules/

# The embedded dist is built only inside Dagger; keep the placeholder.
internal/static/dist/*
!internal/static/dist/.gitkeep

# Runtime
data/
.env

# OS
.DS_Store
```

- [ ] **Step 2: Copy the canonical golangci config**

Run:
```bash
cp /Users/john/code/git/style-guides/.golangci.yml .golangci.yml
```

- [ ] **Step 3: Commit**

```bash
git add .gitignore .golangci.yml
git commit -m "chore: add gitignore and golangci config"
```

---

### Task 2: Go module, version package, embedded static placeholder

**Files:**
- Create: `go.mod` (via `go mod init`)
- Create: `version/version.go`
- Create: `internal/static/static.go`
- Create: `internal/static/dist/.gitkeep`

- [ ] **Step 1: Initialize the module**

Run:
```bash
go mod init github.com/jwhumphries/bandwidth && go mod edit -go=1.26
```

- [ ] **Step 2: Write `version/version.go`**

```go
// Package version exposes the build-time injected version string.
package version

// Version is overridden at release build time via
// -ldflags "-X github.com/jwhumphries/bandwidth/version.Version=...".
var Version = "dev"
```

- [ ] **Step 3: Write the embedded static package**

Create `internal/static/dist/.gitkeep` (empty file), then `internal/static/static.go`:

```go
// Package static embeds the compiled frontend assets.
package static

import "embed"

// Dist holds the built frontend. Locally it contains only a placeholder;
// the real assets are copied in during the Dagger release build.
//
//go:embed all:dist
var Dist embed.FS
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./...`
Expected: exits 0, no output.

- [ ] **Step 5: Commit**

```bash
git add go.mod version/ internal/static/
git commit -m "feat: go module with version and embedded static packages"
```

---

### Task 3: Healthz handler (TDD)

**Files:**
- Create: `internal/handlers/health_test.go`
- Create: `internal/handlers/health.go`

- [ ] **Step 1: Add Echo v5 and write the failing test**

Run: `go get github.com/labstack/echo/v5@latest`

Create `internal/handlers/health_test.go`:

```go
package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestHealthz(t *testing.T) {
	e := echo.New()
	e.GET("/healthz", Healthz)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"status":"ok"`) {
		t.Fatalf("body = %q, want it to contain %q", body, `"status":"ok"`)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/handlers/`
Expected: FAIL — compile error `undefined: Healthz`.

- [ ] **Step 3: Write the handler**

Create `internal/handlers/health.go`:

```go
// Package handlers contains the HTTP handlers for the BandWidth server.
package handlers

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// Healthz reports server liveness.
func Healthz(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
```

Note: Echo v5.1.1 is current as of this plan. If the handler signature does not compile, check `go doc github.com/labstack/echo/v5.HandlerFunc` and adapt — but only `echo.New`, `e.GET`, `c.JSON`, `c.Request`, `c.Response`, and `middleware.Recover` are used in this entire plan, all stable across v4/v5.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/handlers/`
Expected: `ok  github.com/jwhumphries/bandwidth/internal/handlers`

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/handlers/
git commit -m "feat: healthz endpoint"
```

---

### Task 4: SPA fallback handler (TDD)

**Files:**
- Create: `internal/handlers/spa_test.go`
- Create: `internal/handlers/spa.go`

The SPA handler serves files from an `fs.FS` and falls back to `index.html` for unknown paths (client-side routes). It takes `fs.FS` as a parameter so tests use `fstest.MapFS` while production passes the embedded build.

- [ ] **Step 1: Write the failing test**

Create `internal/handlers/spa_test.go`:

```go
package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/labstack/echo/v5"
)

func TestSPA(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app shell</html>")},
		"assets/app.js": {Data: []byte("console.log('hi');")},
	}
	e := echo.New()
	RegisterSPA(e, fsys)

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "root serves index", path: "/", want: "app shell"},
		{name: "existing asset served", path: "/assets/app.js", want: "console.log"},
		{name: "client route falls back to index", path: "/songs/42", want: "app shell"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if body := rec.Body.String(); !strings.Contains(body, tt.want) {
				t.Fatalf("body = %q, want it to contain %q", body, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/handlers/`
Expected: FAIL — compile error `undefined: RegisterSPA`.

- [ ] **Step 3: Write the handler**

Create `internal/handlers/spa.go`:

```go
package handlers

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
)

// RegisterSPA serves the frontend build from fsys on the catch-all route,
// falling back to index.html for paths that don't exist so client-side
// routing works on hard refresh and deep links.
func RegisterSPA(e *echo.Echo, fsys fs.FS) {
	fileServer := http.FileServerFS(fsys)
	e.GET("/*", func(c echo.Context) error {
		req := c.Request()
		path := strings.TrimPrefix(req.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(fsys, path); err != nil {
				req.URL.Path = "/" // fall back to index.html
			}
		}
		fileServer.ServeHTTP(c.Response(), req)
		return nil
	})
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/handlers/`
Expected: `ok` with all three subtests passing (`go test -v` to see them).

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/
git commit -m "feat: SPA fallback handler"
```

---

### Task 5: CLI, config, and server wiring

**Files:**
- Create: `cmd/bandwidth/server_test.go`
- Create: `cmd/bandwidth/main.go`
- Create: `cmd/bandwidth/server.go`

- [ ] **Step 1: Add deps and write the failing wiring test**

Run: `go get github.com/spf13/cobra@latest github.com/spf13/viper@latest`

Create `cmd/bandwidth/server_test.go`:

```go
package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewEchoServesHealthz(t *testing.T) {
	e := newEcho(slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./cmd/bandwidth/`
Expected: FAIL — compile error `undefined: newEcho`.

- [ ] **Step 3: Write `cmd/bandwidth/main.go`**

```go
// Command bandwidth runs the BandWidth server.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jwhumphries/bandwidth/version"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	initConfig()
	return &cobra.Command{
		Use:           "bandwidth",
		Short:         "Practice tracking for musicians and bands",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(*cobra.Command, []string) error {
			return runServer()
		},
	}
}

// initConfig wires Viper to BANDWIDTH_* environment variables.
// Keys: port (BANDWIDTH_PORT), log_level (BANDWIDTH_LOG_LEVEL).
func initConfig() {
	viper.SetDefault("port", ":8080")
	viper.SetDefault("log_level", "info")
	viper.SetEnvPrefix("BANDWIDTH")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
}
```

- [ ] **Step 4: Write `cmd/bandwidth/server.go`**

The stdlib `http.Server` hosts Echo as a plain `http.Handler` — version-proof and gives us explicit graceful shutdown.

```go
package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/spf13/viper"

	"github.com/jwhumphries/bandwidth/internal/handlers"
	"github.com/jwhumphries/bandwidth/internal/static"
	"github.com/jwhumphries/bandwidth/version"
)

func runServer() error {
	logger := newLogger(viper.GetString("log_level"))

	srv := &http.Server{
		Addr:              viper.GetString("port"),
		Handler:           newEcho(logger),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting server", "addr", srv.Addr, "version", version.Version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func newEcho(logger *slog.Logger) *echo.Echo {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(requestLogger(logger))

	e.GET("/healthz", handlers.Healthz)

	dist, err := fs.Sub(static.Dist, "dist")
	if err != nil {
		panic(err) // embed is checked at compile time; this cannot fail
	}
	handlers.RegisterSPA(e, dist)
	return e
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}

func requestLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			req := c.Request()
			logger.Info("request",
				"method", req.Method,
				"path", req.URL.Path,
				"status", c.Response().Status,
				"duration", time.Since(start).String(),
			)
			return err
		}
	}
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./...`
Expected: `ok` for `cmd/bandwidth` and `internal/handlers`.

- [ ] **Step 6: Manual smoke test**

Run:
```bash
go run ./cmd/bandwidth & sleep 2 && curl -s localhost:8080/healthz; kill %1
```
Expected: `{"status":"ok"}` and a JSON request log line on stdout.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum cmd/
git commit -m "feat: server wiring with cobra/viper config and graceful shutdown"
```

---

### Task 6: Frontend scaffold with first test (TDD)

**Files:**
- Create: `frontend/package.json`, `frontend/index.html`, `frontend/vite.config.ts`, `frontend/vitest.config.ts`, `frontend/tsconfig.json`, `frontend/.prettierignore`
- Create: `frontend/eslint.config.js`, `frontend/.prettierrc.json` (copied from style-guides)
- Create: `frontend/src/index.css`, `frontend/src/main.tsx`, `frontend/src/App.tsx`, `frontend/src/test/setup.ts`
- Create: `frontend/src/pages/HomePage.test.tsx`, `frontend/src/pages/HomePage.tsx`

- [ ] **Step 1: Write `frontend/package.json` and install dependencies**

```json
{
  "name": "bandwidth-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "test": "vitest run",
    "test:watch": "vitest",
    "typecheck": "tsc --noEmit",
    "lint": "eslint .",
    "format": "prettier --write .",
    "format:check": "prettier --check ."
  }
}
```

Run:
```bash
cd frontend
bun add react react-dom react-router
bun add -d typescript vite @vitejs/plugin-react @types/react @types/react-dom \
  tailwindcss @tailwindcss/vite daisyui \
  vitest jsdom @testing-library/react @testing-library/jest-dom \
  eslint @eslint/js typescript-eslint eslint-config-prettier eslint-plugin-prettier prettier
cd ..
```
Expected: `bun.lock` created, no errors.

- [ ] **Step 2: Copy canonical lint/format configs**

Run:
```bash
cp /Users/john/code/git/style-guides/eslint.config.js frontend/eslint.config.js
cp /Users/john/code/git/style-guides/.prettierrc.json frontend/.prettierrc.json
```

Create `frontend/.prettierignore`:

```
dist/
node_modules/
bun.lock
```

- [ ] **Step 3: Write the build configs**

`frontend/vite.config.ts` (Vite builds to its default `frontend/dist`, which is gitignored; the release pipeline copies it into `internal/static/dist` inside Dagger):

```ts
import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import {defineConfig} from 'vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 3000,
    proxy: {
      '/api': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
    },
  },
});
```

`frontend/vitest.config.ts`:

```ts
import react from '@vitejs/plugin-react';
import {defineConfig} from 'vitest/config';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
  },
});
```

`frontend/tsconfig.json` (browser-bundle settings; strict-family flags copied inline from the style-guides base, same documented deviation as ReadWillBe):

```json
{
  "compilerOptions": {
    "target": "es2022",
    "module": "esnext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "lib": ["dom", "dom.iterable", "esnext"],
    "types": ["vite/client"],
    "strict": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "allowUnreachableCode": false,
    "allowUnusedLabels": false,
    "forceConsistentCasingInFileNames": true,
    "isolatedModules": true,
    "useDefineForClassFields": true,
    "skipLibCheck": true,
    "noEmit": true
  },
  "include": ["src", "vite.config.ts", "vitest.config.ts"]
}
```

- [ ] **Step 4: Write the app shell**

`frontend/index.html`:

```html
<!doctype html>
<html lang="en" data-theme="light">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>BandWidth</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

`frontend/src/index.css`:

```css
@import 'tailwindcss';
@plugin "daisyui";
```

`frontend/src/main.tsx`:

```tsx
import {StrictMode} from 'react';
import {createRoot} from 'react-dom/client';
import {BrowserRouter} from 'react-router';
import App from './App';
import './index.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </StrictMode>,
);
```

`frontend/src/App.tsx`:

```tsx
import {Route, Routes} from 'react-router';
import HomePage from './pages/HomePage';

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<HomePage />} />
    </Routes>
  );
}
```

`frontend/src/test/setup.ts`:

```ts
import '@testing-library/jest-dom/vitest';
```

- [ ] **Step 5: Write the failing HomePage test**

`frontend/src/pages/HomePage.test.tsx`:

```tsx
import {render, screen} from '@testing-library/react';
import {describe, expect, it} from 'vitest';
import HomePage from './HomePage';

describe('HomePage', () => {
  it('renders the app name', () => {
    render(<HomePage />);
    expect(screen.getByRole('heading', {name: /bandwidth/i})).toBeInTheDocument();
  });
});
```

- [ ] **Step 6: Run the test to verify it fails**

Run: `cd frontend && bun run test`
Expected: FAIL — cannot resolve `./HomePage`.

- [ ] **Step 7: Write `frontend/src/pages/HomePage.tsx`**

```tsx
export default function HomePage() {
  return (
    <main className="hero bg-base-200 min-h-screen">
      <div className="hero-content text-center">
        <div>
          <h1 className="text-5xl font-bold">BandWidth</h1>
          <p className="py-4">Practice tracking for musicians and bands.</p>
        </div>
      </div>
    </main>
  );
}
```

- [ ] **Step 8: Run all frontend checks**

Run (from `frontend/`): `bun run test && bun run typecheck && bun run lint && bun run format:check && bun run build`
Expected: all pass; `frontend/dist/` produced. If `format:check` flags files, run `bun run format` once and re-check — copied configs and hand-typed files must end up Prettier-clean.

- [ ] **Step 9: Commit**

```bash
cd .. && git add frontend
git commit -m "feat: frontend scaffold with vite, react router, tailwind and daisyui"
```

---

### Task 7: Server status badge on HomePage (TDD)

**Files:**
- Modify: `frontend/src/pages/HomePage.test.tsx`
- Modify: `frontend/src/pages/HomePage.tsx`

This tiny feature proves the dev proxy and API wiring end-to-end.

- [ ] **Step 1: Add failing tests**

Replace `frontend/src/pages/HomePage.test.tsx` with:

```tsx
import {render, screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import HomePage from './HomePage';

describe('HomePage', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ok: true} as unknown as Response),
    );
  });

  it('renders the app name', () => {
    render(<HomePage />);
    expect(screen.getByRole('heading', {name: /bandwidth/i})).toBeInTheDocument();
  });

  it('shows server online once the health check resolves', async () => {
    render(<HomePage />);
    await waitFor(() =>
      expect(screen.getByText(/server online/i)).toBeInTheDocument(),
    );
  });

  it('shows server unreachable when the health check fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('down')));
    render(<HomePage />);
    await waitFor(() =>
      expect(screen.getByText(/server unreachable/i)).toBeInTheDocument(),
    );
  });
});
```

- [ ] **Step 2: Run tests to verify the new ones fail**

Run: `cd frontend && bun run test`
Expected: "renders the app name" passes; the two status tests FAIL (text not found).

- [ ] **Step 3: Implement the badge**

Replace `frontend/src/pages/HomePage.tsx` with:

```tsx
import {useEffect, useState} from 'react';

type ServerStatus = 'checking' | 'ok' | 'unreachable';

export default function HomePage() {
  const [status, setStatus] = useState<ServerStatus>('checking');

  useEffect(() => {
    let cancelled = false;
    fetch('/healthz')
      .then(res => {
        if (!cancelled) setStatus(res.ok ? 'ok' : 'unreachable');
      })
      .catch(() => {
        if (!cancelled) setStatus('unreachable');
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <main className="hero bg-base-200 min-h-screen">
      <div className="hero-content text-center">
        <div>
          <h1 className="text-5xl font-bold">BandWidth</h1>
          <p className="py-4">Practice tracking for musicians and bands.</p>
          <ServerStatusBadge status={status} />
        </div>
      </div>
    </main>
  );
}

function ServerStatusBadge({status}: {status: ServerStatus}) {
  if (status === 'checking') {
    return <span className="badge">Checking…</span>;
  }
  if (status === 'ok') {
    return <span className="badge badge-success">Server online</span>;
  }
  return <span className="badge badge-error">Server unreachable</span>;
}
```

- [ ] **Step 4: Run all frontend checks**

Run (from `frontend/`): `bun run test && bun run typecheck && bun run lint && bun run format:check`
Expected: all pass (3 tests green).

- [ ] **Step 5: Commit**

```bash
cd .. && git add frontend/src/pages/
git commit -m "feat: server status badge on home page"
```

---

### Task 8: Dagger module and justfile

**Files:**
- Create: `dagger.json`, `.dagger/main.go` (+ generated `.dagger/` files)
- Create: `justfile`

- [ ] **Step 1: Initialize the Dagger module**

Run:
```bash
dagger init --sdk go --source .dagger --name bandwidth
```
Expected: `dagger.json` and `.dagger/` created (template `main.go`, `go.mod`, generated `internal/`).

- [ ] **Step 2: Replace `.dagger/main.go` with the pipeline**

```go
// CI/build pipeline for BandWidth. Every check and build runs in a
// container; the justfile recipes are thin wrappers over these functions.
package main

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"dagger/bandwidth/internal/dagger"
)

const (
	goImage     = "golang:1.26-alpine"
	bunImage    = "oven/bun:1-alpine"
	lintImage   = "golangci/golangci-lint:v2-alpine"
	alpineImage = "alpine:3.23"
)

type Bandwidth struct{}

// goBase is a Go toolchain container with the source and module/build caches.
func (m *Bandwidth) goBase(source *dagger.Directory) *dagger.Container {
	return dag.Container().
		From(goImage).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("bandwidth-go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("bandwidth-go-build")).
		WithDirectory("/app", source).
		WithWorkdir("/app")
}

// bunBase is a bun container with the frontend source and dependencies installed.
func (m *Bandwidth) bunBase(frontend *dagger.Directory) *dagger.Container {
	return dag.Container().
		From(bunImage).
		WithMountedCache("/root/.bun/install/cache", dag.CacheVolume("bandwidth-bun")).
		WithDirectory("/app", frontend).
		WithWorkdir("/app").
		WithExec([]string{"bun", "install", "--frozen-lockfile"})
}

// Check runs every lint, type, format, and test gate in parallel.
func (m *Bandwidth) Check(
	ctx context.Context,
	// +ignore=["**/node_modules", "frontend/dist", "tmp", "bin", "data", ".git"]
	source *dagger.Directory,
) (string, error) {
	checks := map[string]func(context.Context, *dagger.Directory) (string, error){
		"lint-go":       m.LintGo,
		"test-go":       m.TestGo,
		"lint-js":       m.LintJs,
		"typecheck":     m.Typecheck,
		"format-check":  m.FormatCheck,
		"test-frontend": m.TestFrontend,
	}
	eg, ctx := errgroup.WithContext(ctx)
	for name, fn := range checks {
		eg.Go(func() error {
			if _, err := fn(ctx, source); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return "", err
	}
	return "all checks passed", nil
}

// LintGo runs golangci-lint.
func (m *Bandwidth) LintGo(
	ctx context.Context,
	// +ignore=["**/node_modules", "frontend/dist", "tmp", "bin", "data", ".git"]
	source *dagger.Directory,
) (string, error) {
	return dag.Container().
		From(lintImage).
		WithMountedCache("/go/pkg/mod", dag.CacheVolume("bandwidth-go-mod")).
		WithMountedCache("/root/.cache/go-build", dag.CacheVolume("bandwidth-go-build")).
		WithMountedCache("/root/.cache/golangci-lint", dag.CacheVolume("bandwidth-golangci")).
		WithDirectory("/app", source).
		WithWorkdir("/app").
		WithExec([]string{"golangci-lint", "run", "./..."}).
		Stdout(ctx)
}

// TestGo runs the Go tests.
func (m *Bandwidth) TestGo(
	ctx context.Context,
	// +ignore=["**/node_modules", "frontend/dist", "tmp", "bin", "data", ".git"]
	source *dagger.Directory,
) (string, error) {
	return m.goBase(source).
		WithExec([]string{"go", "test", "./..."}).
		Stdout(ctx)
}

// LintJs runs ESLint on the frontend.
func (m *Bandwidth) LintJs(
	ctx context.Context,
	// +ignore=["**/node_modules", "frontend/dist", "tmp", "bin", "data", ".git"]
	source *dagger.Directory,
) (string, error) {
	return m.bunBase(source.Directory("frontend")).
		WithExec([]string{"bun", "run", "lint"}).
		Stdout(ctx)
}

// Typecheck runs the TypeScript compiler in noEmit mode.
func (m *Bandwidth) Typecheck(
	ctx context.Context,
	// +ignore=["**/node_modules", "frontend/dist", "tmp", "bin", "data", ".git"]
	source *dagger.Directory,
) (string, error) {
	return m.bunBase(source.Directory("frontend")).
		WithExec([]string{"bun", "run", "typecheck"}).
		Stdout(ctx)
}

// FormatCheck runs Prettier in check mode.
func (m *Bandwidth) FormatCheck(
	ctx context.Context,
	// +ignore=["**/node_modules", "frontend/dist", "tmp", "bin", "data", ".git"]
	source *dagger.Directory,
) (string, error) {
	return m.bunBase(source.Directory("frontend")).
		WithExec([]string{"bun", "run", "format:check"}).
		Stdout(ctx)
}

// TestFrontend runs the Vitest suite.
func (m *Bandwidth) TestFrontend(
	ctx context.Context,
	// +ignore=["**/node_modules", "frontend/dist", "tmp", "bin", "data", ".git"]
	source *dagger.Directory,
) (string, error) {
	return m.bunBase(source.Directory("frontend")).
		WithExec([]string{"bun", "run", "test"}).
		Stdout(ctx)
}

// Fmt runs goimports and returns the formatted source tree.
func (m *Bandwidth) Fmt(
	// +ignore=["**/node_modules", "frontend/dist", "tmp", "bin", "data", ".git"]
	source *dagger.Directory,
) *dagger.Directory {
	return m.goBase(source).
		WithExec([]string{"go", "run", "golang.org/x/tools/cmd/goimports@latest",
			"-w", "-local", "github.com/jwhumphries/bandwidth", "."}).
		Directory("/app")
}

// Format runs Prettier in write mode and returns the formatted frontend tree.
func (m *Bandwidth) Format(
	// +ignore=["**/node_modules", "frontend/dist", "tmp", "bin", "data", ".git"]
	source *dagger.Directory,
) *dagger.Directory {
	return m.bunBase(source.Directory("frontend")).
		WithExec([]string{"bun", "run", "format"}).
		Directory("/app").
		WithoutDirectory("node_modules")
}

// BuildFrontend compiles the frontend and returns the dist directory.
func (m *Bandwidth) BuildFrontend(
	// +ignore=["**/node_modules", "frontend/dist", "tmp", "bin", "data", ".git"]
	source *dagger.Directory,
) *dagger.Directory {
	return m.bunBase(source.Directory("frontend")).
		WithExec([]string{"bun", "run", "build"}).
		Directory("/app/dist")
}

// Release builds the production container: frontend dist embedded in a
// static Go binary on Alpine with a nonroot user.
func (m *Bandwidth) Release(
	// +ignore=["**/node_modules", "frontend/dist", "tmp", "bin", "data", ".git"]
	source *dagger.Directory,
	// +optional
	// +default="dev"
	version string,
) *dagger.Container {
	binary := m.goBase(source).
		WithDirectory("/app/internal/static/dist", m.BuildFrontend(source)).
		WithEnvVariable("CGO_ENABLED", "0").
		WithExec([]string{"go", "build",
			"-ldflags", "-s -w -X github.com/jwhumphries/bandwidth/version.Version=" + version,
			"-o", "/out/bandwidth", "./cmd/bandwidth"}).
		File("/out/bandwidth")

	return dag.Container().
		From(alpineImage).
		WithExec([]string{"apk", "add", "--no-cache", "ca-certificates", "tzdata"}).
		WithExec([]string{"addgroup", "-S", "app"}).
		WithExec([]string{"adduser", "-S", "-G", "app", "app"}).
		WithFile("/usr/local/bin/bandwidth", binary).
		WithUser("app").
		WithExposedPort(8080).
		WithEntrypoint([]string{"bandwidth"})
}
```

- [ ] **Step 3: Add errgroup and regenerate**

Run:
```bash
cd .dagger && go get golang.org/x/sync/errgroup && cd ..
dagger develop
dagger functions
```
Expected: function list includes `check`, `lint-go`, `test-go`, `lint-js`, `typecheck`, `format-check`, `test-frontend`, `fmt`, `format`, `build-frontend`, `release`.

- [ ] **Step 4: Write the `justfile`**

```just
# List available commands
default:
    @just --list

# Start the local dev loop (Go API :8080 + Vite :3000)
dev:
    ./scripts/develop.sh

# Run all checks in parallel inside one Dagger session
check:
    dagger call check --source .

# Go linting (golangci-lint)
lint-go:
    dagger call lint-go --source .

# ESLint
lint-js:
    dagger call lint-js --source .

# TypeScript type checking
typecheck:
    dagger call typecheck --source .

# Go tests
test:
    dagger call test-go --source .

# Frontend tests (Vitest)
test-frontend:
    dagger call test-frontend --source .

# Prettier check
format-check:
    dagger call format-check --source .

# Format Go code (goimports)
fmt:
    dagger call fmt --source . export --path .

# Format frontend code (Prettier)
format:
    dagger call format --source . export --path frontend

# Build the frontend dist
build-frontend:
    dagger call build-frontend --source . export --path frontend/dist

# Build the production container image as a tarball
build version=`git rev-parse --short HEAD`:
    mkdir -p tmp
    dagger call release --source . --version {{version}} export --path tmp/bandwidth-image.tar

# Remove build artifacts
clean:
    rm -rf tmp frontend/dist frontend/node_modules
```

(Note: `scripts/develop.sh` is created in Task 9; `just dev` will not work until then.)

- [ ] **Step 5: Run the full check suite via Dagger**

Run: `just check`
Expected: `all checks passed`. First run is slow (image pulls); failures name the offending gate (e.g. `lint-go: ...`). Fix any findings — golangci-lint may flag things host `go test` did not.

- [ ] **Step 6: Verify the release build**

Run: `just build && ls -lh tmp/bandwidth-image.tar`
Expected: tarball exists (the full frontend-embed + static-binary path works).

- [ ] **Step 7: Commit**

```bash
git add dagger.json .dagger justfile
git commit -m "ci: dagger pipeline and justfile"
```

---

### Task 9: Dev loop (air + vite)

**Files:**
- Create: `.air.toml`
- Create: `scripts/develop.sh`

- [ ] **Step 1: Write `.air.toml`**

```toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/bandwidth ./cmd/bandwidth"
bin = "tmp/bandwidth"
include_ext = ["go"]
exclude_dir = ["frontend", "tmp", "bin", "data", ".dagger", "docs", "scripts"]

[misc]
clean_on_exit = true
```

- [ ] **Step 2: Write `scripts/develop.sh`**

```bash
#!/usr/bin/env bash
# Local dev loop: Go API via air (:8080) + Vite dev server (:3000).
# Vite proxies /api and /healthz to the Go server; open http://localhost:3000.
set -euo pipefail
cd "$(dirname "$0")/.."

if ! command -v air >/dev/null 2>&1; then
  echo "air not found — install with: go install github.com/air-verse/air@latest" >&2
  exit 1
fi

trap 'kill 0' EXIT

(cd frontend && bun install && exec bun run dev) &
air &
wait
```

Run: `chmod +x scripts/develop.sh`

- [ ] **Step 3: Verify the dev loop**

Run `just dev`, wait for both servers, then in another shell:
```bash
curl -s localhost:3000/healthz
curl -s localhost:3000/ | grep -o "<title>BandWidth</title>"
```
Expected: `{"status":"ok"}` (proxied through Vite) and the title tag. Stop with Ctrl-C — both processes must exit.

- [ ] **Step 4: Commit**

```bash
git add .air.toml scripts/
git commit -m "feat: hot-reload dev loop with air and vite"
```

---

### Task 10: GitHub Actions and Renovate

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `.github/renovate.json`

- [ ] **Step 1: Write `.github/workflows/ci.yml`**

```yaml
name: ci

on:
  push:
    branches: ["**"]
  pull_request:
    branches: [main]

jobs:
  check:
    runs-on: ubuntu-latest
    env:
      DAGGER_NO_NAG: "1"
    steps:
      - uses: actions/checkout@v4

      - name: Check
        uses: dagger/dagger-for-github@v7
        with:
          version: "latest"
          verb: call
          module: .dagger
          args: check --source=.

  release:
    runs-on: ubuntu-latest
    env:
      DAGGER_NO_NAG: "1"
    steps:
      - uses: actions/checkout@v4

      - name: Release build
        uses: dagger/dagger-for-github@v7
        with:
          version: "latest"
          verb: call
          module: .dagger
          args: release --source=. --version=${{ github.sha }}
```

- [ ] **Step 2: Write `.github/renovate.json`**

```json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": ["config:recommended"],
  "schedule": ["before 9am on friday"]
}
```

- [ ] **Step 3: Commit**

```bash
git add .github/
git commit -m "ci: github actions workflow and renovate config"
```

(CI can only be fully verified after pushing to GitHub; `just check` runs the identical Dagger function locally.)

---

### Task 11: Documentation and final verification

**Files:**
- Create: `README.md`, `AGENTS.md`, `CLAUDE.md`

- [ ] **Step 1: Write `README.md`**

````markdown
# BandWidth

Practice tracking for musicians and bands. Track songs through
Not Learned → Learning → Learned → Nailed!, log practice days, organize
songs into folders, and share songs and statuses with your band.

Design: [docs/superpowers/specs/2026-06-10-bandwidth-design.md](docs/superpowers/specs/2026-06-10-bandwidth-design.md)

## Stack

Go + Echo + SQLite (GORM) backend; React 19 + TypeScript + Vite +
Tailwind CSS/DaisyUI frontend; installable PWA; single container on fly.io.

## Development

Requires `go`, `bun`, `just`, `dagger`, and `air`
(`go install github.com/air-verse/air@latest`).

```bash
just dev     # hot-reload dev loop: Go API :8080 + Vite :3000
just check   # all lints, type checks, and tests (in Dagger)
just build   # production container image tarball
```

`just --list` shows all recipes.
````

- [ ] **Step 2: Write `AGENTS.md`**

```markdown
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
- `.dagger/` — CI pipeline; the justfile is a thin wrapper over it

## Style guides & documented deviations

This repo follows the conventions from the `style-guides` repo (local copy
at `/Users/john/code/git/style-guides/` on the primary dev machine).
Deviations:

- **Parallel `Check` Dagger function instead of one CI job per task**
  (same trade-off as ReadWillBe: faster, coarser GitHub status).
- **Frontend tooling configs live in `frontend/`** (`eslint.config.js`,
  `.prettierrc.json`, `tsconfig.json`), not the repo root, because the
  frontend is a self-contained Vite app with its own package.json.
- **`tsconfig.json` does not extend `tsconfig.base.json`** — browser-bundle
  settings differ; the strict-family flags are copied inline.

## Testing

- Go: table-driven tests alongside source (`*_test.go`); handlers tested
  through `e.ServeHTTP` with `httptest`; SPA handler takes an `fs.FS` so
  tests use `fstest.MapFS`.
- Frontend: Vitest + Testing Library (jsdom); setup in
  `frontend/src/test/setup.ts`.
```

- [ ] **Step 3: Write `CLAUDE.md`**

```markdown
# CLAUDE.md

See [AGENTS.md](AGENTS.md) for guidance to AI agents working in this repo.
```

- [ ] **Step 4: Final verification**

Run: `just check`
Expected: `all checks passed`.

Run: `git status --porcelain`
Expected: only the three new docs files staged/untracked; nothing else dirty.

- [ ] **Step 5: Commit**

```bash
git add README.md AGENTS.md CLAUDE.md
git commit -m "docs: readme, agents guide, and claude pointer"
```

---

## Done criteria

- `just dev` serves the DaisyUI home page at `:3000` with a green "Server online" badge (proxy → Go `/healthz` works).
- `just check` passes: golangci-lint, ESLint, Prettier, tsc, Go tests, Vitest.
- `just build` produces a container tarball with the frontend embedded in the binary.
- CI workflow runs the same Dagger functions on push.
- Next: Plan 2 (Auth + Profile) gets written against this skeleton.
