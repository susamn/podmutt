package artwork

import (
	"image/color"
	"image/draw"
	"testing"
)

func TestLogoDecodesIndependentImagesForExistingRenderers(t *testing.T) {
	first, err := Logo()
	if err != nil {
		t.Fatal(err)
	}
	second, err := Logo()
	if err != nil {
		t.Fatal(err)
	}
	before := second.At(0, 0)
	first.(draw.Image).Set(0, 0, color.RGBA{255, 0, 255, 255})
	if first.At(0, 0) == before || second.At(0, 0) != before {
		t.Fatal("logo images share mutable pixels")
	}
	if text := ASCII(second, 16, 6); text == "" || text == "[no artwork]" {
		t.Fatal("logo cannot use ASCII renderer", text)
	}
	packet, lines := Kitty(second, 1, 16, 6)
	if packet == "" || len(lines) == 0 {
		t.Fatal("logo cannot use existing Kitty path")
	}
}
