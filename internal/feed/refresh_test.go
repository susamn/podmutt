package feed

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"podmutt/internal/store"
)

var ctx = context.Background()

func database(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "podcasts.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
func addFeed(t *testing.T, db *store.Store, address string) store.Feed {
	t.Helper()
	feed := store.Feed{URL: address, Title: "Cached title", Subscribed: true}
	id, err := db.UpsertFeed(ctx, feed)
	if err != nil {
		t.Fatal(err)
	}
	feed.ID = id
	return feed
}
func readFeed(t *testing.T, db *store.Store) store.Feed {
	t.Helper()
	feeds, err := db.Feeds(ctx, true)
	if err != nil || len(feeds) != 1 {
		t.Fatalf("feeds: %+v, %v", feeds, err)
	}
	return feeds[0]
}

func TestConditionalRefreshPreservesHistory(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "podmutt/dev" {
			t.Error("missing user agent")
		}
		switch requests.Add(1) {
		case 1:
			w.Header().Set("ETag", `"v1"`)
			w.Header().Set("Last-Modified", "Tue, 03 Jan 2023 10:00:00 GMT")
		case 2:
			if r.Header.Get("If-None-Match") != `"v1"` || r.Header.Get("If-Modified-Since") == "" {
				t.Error("missing cache validators")
			}
			w.Header().Set("ETag", `"v2"`)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		fmt.Fprint(w, rssFixture)
	}))
	defer server.Close()
	db := database(t)
	feed := addFeed(t, db, server.URL+"/feed")
	refresher := New(db, nil, 0)
	result := refresher.Refresh(ctx, feed)
	if result.Error != nil || result.EpisodeCount != 2 || result.NotModified {
		t.Fatalf("first refresh: %+v", result)
	}
	items, err := db.Episodes(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	id := items[0].ID
	if err := db.Checkpoint(ctx, id, 42); err != nil {
		t.Fatal(err)
	}
	if err := db.SetCompleted(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceQueue(ctx, []int64{id}); err != nil {
		t.Fatal(err)
	}
	feed = readFeed(t, db)
	result = refresher.Refresh(ctx, feed)
	if result.Error != nil || !result.NotModified {
		t.Fatalf("conditional refresh: %+v", result)
	}
	feed = readFeed(t, db)
	if feed.ETag != `"v2"` || feed.LastModified == "" {
		t.Fatalf("304 headers: %+v", feed)
	}
	result = refresher.Refresh(ctx, feed)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	feed = readFeed(t, db)
	if feed.ETag != "" || feed.LastModified != "" {
		t.Fatal("200 retained stale validators")
	}
	queue, err := db.Queue(ctx)
	if err != nil || len(queue) != 1 || queue[0].ID != id || !queue[0].Completed || queue[0].PositionSeconds != 42 {
		t.Fatalf("refresh changed history/queue: %+v, %v", queue, err)
	}
}

func TestRedirectUsesFinalBaseWithoutChangingSubscription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/nested/feed.xml", http.StatusFound)
			return
		}
		fmt.Fprint(w, rssFixture)
	}))
	defer server.Close()
	db := database(t)
	feed := addFeed(t, db, server.URL+"/old")
	originalClient := server.Client()
	result := New(db, originalClient, 0).Refresh(ctx, feed)
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	if originalClient.CheckRedirect != nil {
		t.Fatal("caller client was mutated")
	}
	if readFeed(t, db).URL != feed.URL {
		t.Fatal("redirect changed subscription identity")
	}
	items, err := db.Episodes(ctx, feed.ID)
	if err != nil || items[0].ArtworkURL != server.URL+"/nested/episode.png" {
		t.Fatalf("relative artwork: %+v, %v", items, err)
	}
}

func TestHTTPFailuresKeepCachedMetadata(t *testing.T) {
	for _, mode := range []string{"status", "rate-limit", "unexpected-304", "size-header", "size-stream", "xml", "redirect-scheme", "redirect-credentials", "redirect-loop"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "status":
					w.WriteHeader(500)
				case "rate-limit":
					w.WriteHeader(429)
				case "unexpected-304":
					w.WriteHeader(304)
				case "size-header":
					w.Header().Set("Content-Length", fmt.Sprint(MaxBytes+1))
				case "size-stream":
					w.(http.Flusher).Flush()
					fmt.Fprint(w, strings.Repeat("x", MaxBytes+1))
				case "xml":
					fmt.Fprint(w, "<rss>")
				case "redirect-scheme":
					http.Redirect(w, r, "file:///tmp/feed", 302)
				case "redirect-credentials":
					http.Redirect(w, r, "http://user:secret@example.test/feed", 302)
				case "redirect-loop":
					http.Redirect(w, r, "/loop", 302)
				}
			}))
			defer server.Close()
			db := database(t)
			feed := addFeed(t, db, server.URL+"/feed")
			if err := db.UpsertEpisodes(ctx, feed.ID, []store.Episode{{GUID: "cached", AudioURL: server.URL + "/cached.mp3"}}); err != nil {
				t.Fatal(err)
			}
			result := New(db, nil, 0).Refresh(ctx, feed)
			if result.Error == nil {
				t.Fatal("HTTP failure ignored")
			}
			if strings.Contains(result.Error.Error(), "secret") {
				t.Fatal("error disclosed credentials")
			}
			if got := readFeed(t, db); got.Title != "Cached title" || got.ETag != "" {
				t.Fatalf("failure changed cache: %+v", got)
			}
			if items, err := db.Episodes(ctx, feed.ID); err != nil || len(items) != 1 || items[0].GUID != "cached" {
				t.Fatalf("failure lost cached episodes: %+v, %v", items, err)
			}
		})
	}
}
