package library

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"podmutt/internal/feed"
)

func TestPreviewIsReadOnlyAndExplicitSubscriptionPreservesHistory(t *testing.T) {
	db := database(t)
	fetcher := &fixtureFetcher{}
	s := New(db, fetcher)
	preview, err := s.Preview(ctx, "https://example.test/rss#fragment")
	if err != nil {
		t.Fatal(err)
	}
	feeds, err := db.Feeds(ctx, false)
	if err != nil || len(feeds) != 0 {
		t.Fatal("preview stored or subscribed a feed", feeds, err)
	}
	id, err := s.SubscribePreview(ctx, preview)
	if err != nil {
		t.Fatal(err)
	}
	items, err := db.Episodes(ctx, id)
	if err != nil || len(items) != 2 {
		t.Fatal(items, err)
	}
	if err := db.Checkpoint(ctx, items[0].ID, 42); err != nil {
		t.Fatal(err)
	}
	if err := db.SetCompleted(ctx, items[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Subscribe(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	preview.Fetched.Document.Title = "must not replace existing metadata"
	again, err := s.SubscribePreview(ctx, preview)
	if err != nil || again != id || fetcher.calls != 1 {
		t.Fatal(again, err, fetcher.calls)
	}
	restored, err := db.Episode(ctx, items[0].ID)
	if err != nil || !restored.Completed || restored.PositionSeconds != 42 {
		t.Fatal(restored, err)
	}
	if _, err := s.SubscribePreview(ctx, feed.Resolved{URL: "file:///secret"}); err == nil {
		t.Fatal("invalid URL accepted")
	}
	if _, err := s.SubscribePreview(ctx, feed.Resolved{URL: preview.URL, Fetched: feed.Fetched{NotModified: true}}); err == nil {
		t.Fatal("empty preview accepted")
	}
	fetcher.err = failure
	if _, err := s.Preview(ctx, preview.URL); err == nil {
		t.Fatal("preview error hidden")
	}
}
func TestWebsiteAddStoresResolvedRSSURLOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/site" {
			fmt.Fprint(w, `<html><head><link rel="alternate" type="application/rss+xml" href="/rss"></head></html>`)
		} else {
			fmt.Fprint(w, `<rss><channel><title>From website</title><item><title>Episode</title><enclosure url="/audio.mp3"/></item></channel></rss>`)
		}
	}))
	defer server.Close()
	db := database(t)
	s := New(db, feed.New(db, server.Client(), 0))
	preview, err := s.Preview(ctx, server.URL+"/site")
	if err != nil || preview.URL != server.URL+"/rss" {
		t.Fatal(preview, err)
	}
	id, err := s.Add(ctx, server.URL+"/site")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Add(ctx, server.URL+"/site")
	if err != nil || again != id {
		t.Fatal(again, err)
	}
	feeds, err := db.Feeds(ctx, false)
	if err != nil || len(feeds) != 1 || feeds[0].URL != server.URL+"/rss" || !feeds[0].Subscribed {
		t.Fatal(feeds, err)
	}
}
