package app

import (
	"context"
	"errors"
	"image"
	"image/color"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi/kitty"
	"podmutt/internal/feed"
	"podmutt/internal/ui"
)

type artFake struct {
	calls         int
	episode, show string
	ctx           context.Context
	fail          bool
}

func (f *artFake) Image(ctx context.Context, episode, show string) (image.Image, error) {
	f.calls++
	f.episode, f.show, f.ctx = episode, show, ctx
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if f.fail {
		return nil, errors.New("offline")
	}
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.White)
		}
	}
	return img, nil
}
func artModel(t *testing.T, s *session) (libraryShell, *artFake) {
	t.Helper()
	seededLibrary(t, s)
	m, err := newLibrary(s, feed.New(s.store, nil, 0))
	if err != nil {
		t.Fatal(err)
	}
	updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 56})
	m.Model = m.Model.WithFocus(ui.Episodes)
	for i := range m.rows {
		m.rows[i].ArtworkURL = "https://example.test/art/" + strconv.Itoa(i)
	}
	m.snapshot.Feeds[0].ArtworkURL = "https://example.test/show-art"
	f := &artFake{}
	m.art.fetcher = f
	return m, f
}
func TestArtworkAsyncSelectionFallbackFailureAndResize(t *testing.T) {
	librarySession(t, func(s *session) {
		m, f := artModel(t, s)
		m, cmd := m.syncArt()
		if cmd == nil || f.calls != 0 {
			t.Fatal("not async")
		}
		initial := cmd
		m.Model = m.Model.Select(ui.Episodes, strconv.FormatInt(m.rows[1].ID, 10))
		m, cmd = m.syncArt()
		if cmd == nil {
			t.Fatal("selection not loaded")
		}
		updateLibrary(&m, initial())
		if m.art.image != nil {
			t.Fatal("stale image applied")
		}
		updateLibrary(&m, cmd())
		if m.art.image == nil || f.show != "https://example.test/show-art" || f.episode != m.rows[1].ArtworkURL {
			t.Fatal("source/fallback identity")
		}
		if !strings.Contains(m.View().Content, "@@@@") {
			t.Fatal("ASCII not visible")
		}
		m, cmd = m.syncArt()
		if cmd != nil {
			t.Fatal("reloaded completed art")
		}
		updateLibrary(&m, tea.WindowSizeMsg{Width: 80, Height: 24})
		if strings.Contains(m.View().Content, "@@@@") {
			t.Fatal("compact art not hidden")
		}
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 56})
		if !strings.Contains(m.View().Content, "@@@@") {
			t.Fatal("resize lost cached art")
		}
		f.fail = true
		m.Model = m.Model.Select(ui.Episodes, strconv.FormatInt(m.rows[0].ID, 10))
		m, cmd = m.syncArt()
		updateLibrary(&m, cmd())
		if m.art.image != nil || !strings.Contains(m.View().Content, "[no artwork]") {
			t.Fatal("failure not clean placeholder")
		}
	})
}
func TestKittyProbeOrderedDrawAndModalCleanup(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := artModel(t, s)
		m.art.possible = true
		updateLibrary(&m, tea.TerminalVersionMsg{Name: "kitty(0.40.1)"})
		if m.probeArt() == nil {
			t.Fatal("probe missing")
		}
		updateLibrary(&m, uv.KittyGraphicsEvent{Options: kitty.Options{ID: int(m.art.id)}, Payload: []byte("OK")})
		if !m.art.kitty || !m.art.probed {
			t.Fatal("positive response ignored")
		}
		m, cmd := m.syncArt() // Update's deferred sync may already have queued a load.
		if cmd == nil {       // execute the outstanding command explicitly with its generation
			updateLibrary(&m, artLoaded{m.art.generation, image.NewRGBA(image.Rect(0, 0, 4, 4)), nil})
		} else {
			updateLibrary(&m, cmd())
		}
		if !m.art.inFlight || m.art.sent == "" {
			t.Fatal("no serialized graphics draw")
		}
		m.Model = m.Model.WithDialog(&ui.Dialog{Title: "modal"})
		m, cmd = m.syncArt()
		if cmd != nil {
			t.Fatal("overlapping raw operation")
		}
		updateLibrary(&m, artAcknowledged{})
		if m.art.sent != "" || !m.art.inFlight {
			t.Fatal("latest desired deletion not coalesced")
		}
		updateLibrary(&m, artAcknowledged{})
		if m.art.inFlight {
			t.Fatal("empty state redrawn")
		}
		updateLibrary(&m, artProbeTimeout{})
		if !m.art.kitty {
			t.Fatal("late timeout overrode successful probe")
		}
		m.art.probed = false
		m.art.kitty = false
		updateLibrary(&m, artProbeTimeout{})
		if m.art.kitty {
			t.Fatal("probe timeout didn't fallback")
		}
		m.art.possible = false
		if m.probeArt() != nil {
			t.Fatal("queried unknown terminal")
		}
	})
}
