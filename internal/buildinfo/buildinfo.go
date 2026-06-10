// Package buildinfo exposes the build-time injected version string.
package buildinfo

// Version is overridden at release build time via
// -ldflags "-X github.com/jwhumphries/bandwidth/internal/buildinfo.Version=...".
var Version = "dev"
