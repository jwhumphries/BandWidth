// Package version exposes the build-time injected version string.
package version

// Version is overridden at release build time via
// -ldflags "-X github.com/jwhumphries/bandwidth/version.Version=...".
var Version = "dev"
