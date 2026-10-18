// Package artwork provides bounded artwork loading and terminal-safe previews.
package artwork

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

const MaxBytes = 3 << 20
const MaxPixels = 4_000_000

var ErrImage = errors.New("artwork is malformed, unsupported, or too large")

// Decode checks encoded size and dimensions before allocating decoded pixels.
// Only stdlib JPEG/PNG/GIF are accepted; GIF uses its first frame, never animation.
func Decode(data []byte) (image.Image, error) {
	if len(data) > MaxBytes {
		return nil, ErrImage
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || !validSize(config.Width, config.Height) {
		return nil, ErrImage
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrImage
	}
	if !validSize(img.Bounds().Dx(), img.Bounds().Dy()) {
		return nil, ErrImage
	}
	return img, nil
}
func validSize(width, height int) bool {
	return width > 0 && height > 0 && width <= 4096 && height <= 4096 && int64(width)*int64(height) <= MaxPixels
}

// ASCII samples image luminance with a 2:1 cell aspect ratio. Dimensions are
// bounded even if a caller passes a pathological terminal size. No escapes or
// feed-provided text can enter this projection.
func ASCII(img image.Image, columns, rows int) string {
	columns, rows = min(120, columns), min(60, rows)
	if columns <= 0 || rows <= 0 {
		return ""
	}
	if img == nil || img.Bounds().Empty() {
		return "[no artwork]"[:min(columns, len("[no artwork]"))]
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	columns = min(columns, width)
	lines := max(1, int(float64(columns)*float64(height)/float64(width)/2))
	if lines > rows {
		lines = rows
		columns = max(1, min(columns, int(float64(lines)*float64(width)/float64(height)*2)))
	}
	const ramp = " .:-=+*#%@"
	var output strings.Builder
	for y := 0; y < lines; y++ {
		if y > 0 {
			output.WriteByte('\n')
		}
		for x := 0; x < columns; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x*width/columns, bounds.Min.Y+y*height/lines).RGBA()
			luminance := (299*r + 587*g + 114*b) / 1000
			output.WriteByte(ramp[int(luminance)*(len(ramp)-1)/65535])
		}
	}
	return output.String()
}
