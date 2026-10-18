package artwork

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestKittyConservativeDetectionAndOwnedCleanup(t *testing.T) {
	if !KittySupported("xterm-kitty", "", "", "") || !KittySupported("xterm", "123", "", "") {
		t.Fatal("Kitty not detected")
	}
	for _, env := range [][4]string{{"xterm", "", "", ""}, {"xterm-kitty", "", "tmux", ""}, {"xterm-kitty", "", "", "screen"}} {
		if KittySupported(env[0], env[1], env[2], env[3]) {
			t.Fatal("unsafe passthrough")
		}
	}
	if KittyDelete(17) != "\x1b_Ga=d,d=I,i=17,q=2\x1b\\" {
		t.Fatal("deletion not scoped to owned resources")
	}
}
func TestKittyPlaceholderVersionGate(t *testing.T) {
	for _, name := range []string{"kitty(0.28.0)", "kitty(0.40.1)", "kitty(1.0.0)"} {
		if !KittyVersion(name) {
			t.Fatal(name)
		}
	}
	for _, name := range []string{"kitty(0.27.0)", "kitty(0)", "kitty(foo.40)", "kitty(0.-1)", "xterm(0.40)", "kitty(0.40", "kitty(0.x)", strings.Repeat("x", 129)} {
		if KittyVersion(name) {
			t.Fatal(name)
		}
	}
}
func TestKittyChunkingPNGVirtualPlacementAndCellWidths(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 200, 200))
	for i := range img.Pix {
		img.Pix[i] = byte(rand.IntN(256))
	}
	img.Set(0, 0, color.White)
	packet, rows := Kitty(img, 0x123456, 24, 10)
	if len(rows) != 10 || !strings.Contains(packet, "a=T,f=100,t=d,i=1193046,U=1,c=24,r=10,q=2,") {
		t.Fatal("virtual placement missing")
	}
	var encoded strings.Builder
	chunks := strings.Split(strings.TrimSuffix(packet, "\x1b\\"), "\x1b\\")
	for i, chunk := range chunks {
		parts := strings.SplitN(chunk, ";", 2)
		if len(parts) != 2 || len(parts[1]) > 4096 || len(parts[1])%4 != 0 {
			t.Fatal("bad chunk")
		}
		more := 1
		if i == len(chunks)-1 {
			more = 0
		}
		if !strings.Contains(parts[0], fmt.Sprint("m=", more)) {
			t.Fatal("wrong continuation")
		}
		encoded.WriteString(parts[1])
	}
	data, err := base64.StdEncoding.DecodeString(encoded.String())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(data)
	if err != nil || decoded.Bounds().Dx() > 192 || decoded.Bounds().Dy() > 160 {
		t.Fatal("invalid resized PNG", err)
	}
	for _, row := range rows {
		if ansi.StringWidth(row) != 24 || !strings.Contains(row, "\x1b[38;2;18;52;86m") {
			t.Fatal("placeholder geometry or image ID color")
		}
	}
	for _, id := range []uint32{0, 0x1000000} {
		if packet, rows := Kitty(img, id, 20, 10); packet != "" || rows != nil {
			t.Fatal("invalid ID")
		}
	}
	if packet, _ := Kitty(nil, 1, 1, 1); packet != "" {
		t.Fatal("nil image")
	}
	if packet, _ := Kitty(image.NewRGBA(image.Rect(0, 0, 0, 0)), 1, 1, 1); packet != "" {
		t.Fatal("empty image")
	}
	if packet, _ := Kitty(img, 1, 0, 1); packet != "" {
		t.Fatal("zero columns")
	}
	tall := image.NewRGBA(image.Rect(0, 0, 1, 100))
	if packet, _ := Kitty(tall, 1, 20, 5); packet == "" {
		t.Fatal("tall image")
	}
}
