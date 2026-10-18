package artwork

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi/kitty"
)

// KittySupported deliberately excludes multiplexers: passthrough/placeholder
// support varies. Unknown terminals use ASCII, never speculative APC output.
func KittySupported(term, window, tmux, screen string) bool {
	return (term == "xterm-kitty" || window != "") && tmux == "" && screen == ""
}

// KittyVersion requires Unicode placeholder support, not just basic graphics.
func KittyVersion(name string) bool {
	if len(name) > 128 || !strings.HasPrefix(name, "kitty(") || !strings.HasSuffix(name, ")") {
		return false
	}
	parts := strings.Split(name[6:len(name)-1], ".")
	if len(parts) < 2 {
		return false
	}
	major, e1 := strconv.Atoi(parts[0])
	minor, e2 := strconv.Atoi(parts[1])
	return e1 == nil && e2 == nil && major >= 0 && minor >= 0 && (major > 0 || minor >= 28)
}
func KittyDelete(id uint32) string { return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id) }

// Kitty encodes a small private PNG and virtual placement. Direct transmission
// uses <=4096-byte base64 chunks, no filesystem paths/shared-memory handles.
func Kitty(img image.Image, id uint32, columns, rows int) (string, []string) {
	if img == nil || img.Bounds().Empty() || id == 0 || id > 0xffffff || columns < 1 || rows < 1 {
		return "", nil
	}
	columns = min(columns, 60)
	rows = min(rows, 30)
	// Preserve aspect inside the terminal's approximate 2:1 cell aspect ratio.
	b := img.Bounds()
	w, h := columns*8, rows*16
	if b.Dx()*h > b.Dy()*w {
		h = max(1, b.Dy()*w/b.Dx())
	} else {
		w = max(1, b.Dx()*h/b.Dy())
	}
	scaled := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			scaled.SetNRGBA(x, y, color.NRGBAModel.Convert(img.At(b.Min.X+x*b.Dx()/w, b.Min.Y+y*b.Dy()/h)).(color.NRGBA))
		}
	}
	var pngData bytes.Buffer
	_ = png.Encode(&pngData, scaled)
	data := base64.StdEncoding.EncodeToString(pngData.Bytes())
	var packet strings.Builder
	for len(data) > 0 {
		n := min(4096, len(data))
		more := 0
		if n < len(data) {
			more = 1
		}
		opts := ""
		if packet.Len() == 0 {
			opts = fmt.Sprintf("a=T,f=100,t=d,i=%d,U=1,c=%d,r=%d,q=2,", id, columns, rows)
		}
		fmt.Fprintf(&packet, "\x1b_G%sm=%d;%s\x1b\\", opts, more, data[:n])
		data = data[n:]
	}
	result := make([]string, rows)
	for y := range result {
		var row strings.Builder
		fmt.Fprintf(&row, "\x1b[38;2;%d;%d;%dm", id>>16, (id>>8)&255, id&255)
		for x := 0; x < columns; x++ {
			fmt.Fprintf(&row, "%c%c%c", kitty.Placeholder, kitty.Diacritic(y), kitty.Diacritic(x))
		}
		row.WriteString("\x1b[39m")
		result[y] = row.String()
	}
	return packet.String(), result
}
