package assets

import (
	"bytes"
	"image/png"
	"testing"
)

func TestEmbeddedLogoIsPNGAndIndependent(t *testing.T) {
	first := LogoPNG()
	img, err := png.Decode(bytes.NewReader(first))
	if err != nil || img.Bounds().Dx() != 1024 || img.Bounds().Dy() != 1024 {
		t.Fatal("invalid bundled logo", err)
	}
	first[0] = 0
	if LogoPNG()[0] != 0x89 {
		t.Fatal("caller mutated embedded bytes")
	}
}
