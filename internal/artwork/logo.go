package artwork

import (
	"image"

	"podmutt/assets"
)

// Logo decodes a fresh fallback image from the bundled user-supplied PNG. It
// shares neither mutable pixels nor a working-directory/runtime file dependency.
func Logo() (image.Image, error) { return Decode(assets.LogoPNG()) }
