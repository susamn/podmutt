package artwork

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// chafa must be installed, no multiplexer, and TERM must name a terminal known to support sixel
// PODMUTT_SIXEL=1 forces it on, PODMUTT_SIXEL=0 forces it off.
func SixelAvailable(term, tmux, screen, override string) bool {
	if override == "1" {
		return true;
	}
	if override == "0" || tmux != "" || screen != "" {
		return false
	}
	if _, err := exec.LookPath("chafa"); err != nil {
		return false
	}
	for _, p := range []string{"foot", "mlterm", "contour", "wezterm", "yaft"} {
		if strings.HasPrefix(term, p) {
			return true
		}
	}
	return false
}

func Sixel(ctx context.Context, img image.Image, columns, rows int) string {
	if img == nil || columns < 1 || rows < 1 {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	f, err := os.CreateTemp("", "podmutt-art-*.png")
	if err != nil {
		return ""
	}
	defer os.Remove(f.Name())
	err = png.Encode(f, downscale(img, columns*24, rows*48))
	if f.Close() != nil || err != nil {
		return ""
	}
	cmd := exec.CommandContext(ctx, "chafa", "-f", "sixel", "--probe", "off", "--animate", "off",
		"-s", strconv.Itoa(columns)+"x"+strconv.Itoa(rows), f.Name())
	cmd.Stdin = os.Stdin
	var out bytes.Buffer
	cmd.Stdout = &out
	if cmd.Run() != nil {
		return ""
	}
	s := out.String()
	start, end := strings.Index(s, "\x1bP"), strings.LastIndex(s, "\x1b\\")
	if start < 0 || end < start || end+2-start > 4<<20 {
		return ""
	}
	return s[start : end+2]
}

func downscale(img image.Image, maxW, maxH int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxW && h <= maxH {
		return img
	}
	if w*maxH > h*maxW {
		h, w = max(1, h*maxW/w), maxW
	} else {
		w, h = max(1, w*maxH/h), maxH
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	const n = 4
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, bl, a uint64
			for sy := 0; sy < n; sy++ {
				for sx := 0; sx < n; sx++ {
					px := b.Min.X + (x*n+sx)*b.Dx()/(w*n)
					py := b.Min.Y + (y*n+sy)*b.Dy()/(h*n)
					cr, cg, cb, ca := img.At(px, py).RGBA()
					r, g, bl, a = r+uint64(cr), g+uint64(cg), bl+uint64(cb), a+uint64(ca)
				}
			}
			c := color.RGBA64{uint16(r / (n * n)), uint16(g / (n * n)), uint16(bl / (n * n)), uint16(a / (n * n))}
			out.SetNRGBA(x, y, color.NRGBAModel.Convert(c).(color.NRGBA))
		}
	}
	return out
}
