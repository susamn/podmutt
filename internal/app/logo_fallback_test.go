package app

import (
	"context"
	"errors"
	"image"
	"strconv"
	"testing"

	"podmutt/internal/artwork"
	"podmutt/internal/ui"
)

type imageFetcherFunc func(context.Context, string, string) (image.Image, error)

func (f imageFetcherFunc) Image(ctx context.Context, episode, show string) (image.Image, error) {
	return f(ctx, episode, show)
}

func TestLogoFallbackIsOfflineAndCancellationIsNotSuccess(t *testing.T) {
	fake := &artFake{fail: true}
	img, err := loadArtwork(context.Background(), fake, "", "")
	if err != nil || img == nil || img.Bounds().Dx() != 1024 || fake.calls != 0 {
		t.Fatal("missing covers did not use local PNG", err, fake.calls)
	}
	img, err = loadArtwork(context.Background(), fake, "https://example.test/missing", "")
	if err != nil || img == nil || img.Bounds().Dx() != 1024 || fake.calls != 1 {
		t.Fatal("unavailable cover did not use logo", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if img, err = loadArtwork(ctx, fake, "", ""); img != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("canceled fallback succeeded", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	fetch := imageFetcherFunc(func(context.Context, string, string) (image.Image, error) {
		cancel()
		return nil, artwork.ErrUnavailable
	})
	if img, err = loadArtwork(ctx, fetch, "https://example.test/art", ""); img != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation during fetch became a logo", err)
	}
	fetch = imageFetcherFunc(func(context.Context, string, string) (image.Image, error) { return nil, context.DeadlineExceeded })
	if img, err = loadArtwork(context.Background(), fetch, "https://example.test/art", ""); img != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("deadline became success", err)
	}
	fetch = imageFetcherFunc(func(context.Context, string, string) (image.Image, error) { return nil, nil })
	if img, err = loadArtwork(context.Background(), fetch, "https://example.test/art", ""); err != nil || img == nil {
		t.Fatal("empty successful response has no fallback", err)
	}
}

func TestStaleLogoCannotReplaceNewRemoteCover(t *testing.T) {
	librarySession(t, func(s *session) {
		m, f := artModel(t, s)
		for i := range m.rows {
			m.rows[i].ArtworkURL = ""
		}
		m.snapshot.Feeds[0].ArtworkURL = ""
		m.cancelArt() // discard the constructor/resize command not run by this fixture
		m.art.key, m.art.complete = "", false
		next, logo := m.syncArt()
		m = next
		if logo == nil || f.calls != 0 {
			t.Fatal("logo loading was not asynchronous/offline")
		}
		m.rows[1].ArtworkURL = "https://example.test/new-art"
		m.Model = m.Model.Select(ui.Episodes, strconv.FormatInt(m.rows[1].ID, 10))
		next, remote := m.syncArt()
		m = next
		if remote == nil {
			t.Fatal("new art not scheduled")
		}
		updateLibrary(&m, remote())
		if m.art.image == nil || m.art.image.Bounds().Dx() != 16 || f.calls != 1 {
			t.Fatal("remote cover failed")
		}
		updateLibrary(&m, logo())
		if m.art.image == nil || m.art.image.Bounds().Dx() != 16 {
			t.Fatal("stale logo replaced current art")
		}
	})
}
