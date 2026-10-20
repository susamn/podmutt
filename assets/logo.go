// Package assets embeds the application's existing artwork into standalone builds.
package assets

import (
	_ "embed"
	"slices"
)

//go:embed logo.png
var logoPNG []byte

// LogoPNG returns independent bytes; callers cannot mutate the embedded asset.
func LogoPNG() []byte { return slices.Clone(logoPNG) }
