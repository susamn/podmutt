package artwork

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func pngData(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	for y := 0; y < 4; y++ {
		for x := 4; x < 8; x++ {
			img.Set(x, y, color.White)
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}
func TestDecodeFormatsAndPreallocationLimits(t *testing.T) {
	data := pngData(t)
	img, err := Decode(data)
	if err != nil || img.Bounds().Dx() != 8 {
		t.Fatal(img, err)
	}
	for _, encode := range []func(io.Writer, image.Image) error{func(w io.Writer, i image.Image) error { return jpeg.Encode(w, i, nil) }, func(w io.Writer, i image.Image) error { return gif.Encode(w, i, nil) }} {
		var output bytes.Buffer
		if err := encode(&output, img); err != nil {
			t.Fatal(err)
		}
		if _, err := Decode(output.Bytes()); err != nil {
			t.Fatal(err)
		}
	}
	large := bytes.Clone(data)
	binary.BigEndian.PutUint32(large[16:20], 4096)
	binary.BigEndian.PutUint32(large[20:24], 4096)
	binary.BigEndian.PutUint32(large[29:33], crc32.ChecksumIEEE(large[12:29]))
	for _, bad := range [][]byte{nil, []byte("<svg/>"), bytes.Repeat([]byte{'x'}, MaxBytes+1), large, data[:40]} {
		if _, err := Decode(bad); !errors.Is(err, ErrImage) {
			t.Fatal("unsafe decode accepted", err)
		}
	}
	for _, size := range [][2]int{{0, 1}, {1, 0}, {4097, 1}, {1, 4097}, {2001, 2000}} {
		if validSize(size[0], size[1]) {
			t.Fatal(size)
		}
	}
}
func TestASCIIAspectResizeAndMissingPlaceholder(t *testing.T) {
	img, err := Decode(pngData(t))
	if err != nil {
		t.Fatal(err)
	}
	output := ASCII(img, 8, 8)
	if len(strings.Split(output, "\n")) != 2 || !strings.Contains(output, " ") || !strings.Contains(output, "@") {
		t.Fatal(output)
	}
	for _, dimensions := range [][2]int{{0, 8}, {8, 0}, {1, 1}, {100000, 100000}, {4, 1}} {
		out := ASCII(img, dimensions[0], dimensions[1])
		lines := strings.Split(out, "\n")
		if len(lines) > max(1, min(60, dimensions[1])) {
			t.Fatal("too tall")
		}
		for _, line := range lines {
			if len(line) > max(0, min(120, dimensions[0])) {
				t.Fatal("too wide")
			}
			if strings.Contains(line, "\x1b") {
				t.Fatal("escape emitted")
			}
		}
	}
	tall := image.NewRGBA(image.Rect(2, 3, 4, 103))
	if rows := strings.Split(ASCII(tall, 20, 4), "\n"); len(rows) != 4 {
		t.Fatal(rows)
	}
	if ASCII(nil, 20, 4) != "[no artwork]" || len(ASCII(nil, 3, 1)) != 3 {
		t.Fatal("missing placeholder")
	}
	if ASCII(image.NewRGBA(image.Rectangle{}), 20, 4) != "[no artwork]" {
		t.Fatal("empty image")
	}
}
func TestImageFallbackCacheOwnershipAndFailures(t *testing.T) {
	data := pngData(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/bad":
			fmt.Fprint(w, "malformed")
		case "/large":
			w.Header().Set("Content-Length", fmt.Sprint(MaxBytes+1))
		case "/chunked":
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			w.Write(bytes.Repeat([]byte{'x'}, MaxBytes+1))
		case "/error":
			w.WriteHeader(500)
		case "/redirect":
			http.Redirect(w, r, "https://user:secret@example.test/", 302)
		default:
			w.Write(data)
		}
	}))
	defer server.Close()
	f := New(nil)
	first, err := f.Image(context.Background(), server.URL+"/bad", server.URL+"/good")
	if err != nil {
		t.Fatal(err)
	}
	// Returned mutable image ownership must not corrupt later cache hits.
	first.(*image.NRGBA).Set(0, 0, color.White)
	second, err := f.Image(context.Background(), "", server.URL+"/good")
	if err != nil || requests.Load() != 2 {
		t.Fatal(err, requests.Load())
	}
	r, _, _, _ := second.At(0, 0).RGBA()
	if r != 0 {
		t.Fatal("cached image mutated")
	}
	for _, path := range []string{"/large", "/chunked", "/error", "/redirect", "/bad"} {
		if _, err := f.Image(context.Background(), server.URL+path, server.URL+path); !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "secret") {
			t.Fatal(path, err)
		}
	}
	if _, err := f.Image(context.Background(), "file:///secret", "https://user:secret@example.test"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
func TestLRUBudgetsExpiryAndConcurrentReaders(t *testing.T) {
	data := pngData(t)
	f := New(nil)
	f.budget = len(data) * 2
	f.remember("a", data)
	f.remember("b", data)
	if f.cached("a") == nil {
		t.Fatal("miss")
	}
	f.remember("c", data)
	if f.cached("b") != nil || f.cached("a") == nil || f.bytes > f.budget {
		t.Fatal("LRU budget broken")
	}
	entry := f.cache["a"].Value.(cacheEntry)
	entry.expires = time.Now().Add(-time.Second)
	f.cache["a"].Value = entry
	if f.cached("a") != nil {
		t.Fatal("expired cache hit")
	}
	f.remember("oversized", bytes.Repeat([]byte{'x'}, f.budget+1))
	if f.cached("oversized") != nil {
		t.Fatal("cached oversize entry")
	}
	f.budget = 8 << 20
	for i := 0; i < 40; i++ {
		f.remember(fmt.Sprint(i), data)
	}
	if len(f.cache) > 32 {
		t.Fatal("entry cap ignored")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				f.remember("shared", data)
				f.cached("shared")
			}
		}()
	}
	wg.Wait()
}
func TestCancellationAndRedirectBounds(t *testing.T) {
	f := New(nil)
	f.gate <- struct{}{}
	f.gate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := f.Image(ctx, "https://example.test", ""); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	<-f.gate
	<-f.gate
	canceled, stop := context.WithCancel(context.Background())
	stop()
	if _, err := f.Image(canceled, "", ""); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	ctx, stop2 := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop2()
	if _, err := f.Image(ctx, server.URL, ""); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "https://example.test", nil)
	if err := f.client.CheckRedirect(req, make([]*http.Request, 4)); err == nil {
		t.Fatal("redirect cap ignored")
	}
	sentinel := errors.New("caller redirect limit")
	f = New(&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return sentinel }})
	if err := f.client.CheckRedirect(req, nil); err != sentinel {
		t.Fatal(err)
	}
}
