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
	lintImage   = "golangci/golangci-lint:v2.9.0-alpine"
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

// releaseEntrypoint runs as root just long enough to hand the data volume
// to the app user, then drops privileges. Fly mounts volumes root-owned, so
// without the chown the nonroot server cannot create its SQLite file.
const releaseEntrypoint = `#!/bin/sh
set -e
if [ -d /data ]; then
	chown -R app:app /data
fi
exec su-exec app bandwidth "$@"
`

// Release builds the production container: frontend dist embedded in a
// static Go binary on Alpine, run as a nonroot user via an entrypoint that
// first chowns the /data volume.
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
		WithExec([]string{"apk", "add", "--no-cache", "ca-certificates", "tzdata", "su-exec"}).
		WithExec([]string{"addgroup", "-S", "app"}).
		WithExec([]string{"adduser", "-S", "-G", "app", "app"}).
		WithFile("/usr/local/bin/bandwidth", binary).
		WithNewFile("/usr/local/bin/entrypoint.sh", releaseEntrypoint,
			dagger.ContainerWithNewFileOpts{Permissions: 0o755}).
		WithExposedPort(8080).
		WithEntrypoint([]string{"entrypoint.sh"})
}

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
