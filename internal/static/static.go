// Package static embeds the compiled frontend assets.
package static

import "embed"

// Dist holds the built frontend. Locally it contains only a placeholder;
// the real assets are copied in during the Dagger release build.
//
//go:embed all:dist
var Dist embed.FS
