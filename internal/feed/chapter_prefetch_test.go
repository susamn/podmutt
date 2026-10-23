package feed

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRefreshCachesLinkedChaptersOncePerURL(t *testing.T) {
	var requested atomic.Int32
	var version atomic.Int32
	version.Store(1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed":
			fmt.Fprintf(w, `<rss version="2.0" xmlns:podcast="http://podcastindex.org/namespace/1.0"><channel><title>Show</title><item><guid>1</guid><title>Episode</title><enclosure url="https://example.test/audio" type="audio/mpeg"/><podcast:chapters url="/chapter%d.json" type="application/json+chapters"/></item></channel></rss>`, version.Load())
		case "/chapter1.json", "/chapter2.json":
			requested.Add(1)
			fmt.Fprint(w, `{"chapters":[{"startTime":0,"title":"Intro"},{"startTime":15,"title":"Topic"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	db := database(t)
	feed := addFeed(t, db, server.URL+"/feed")
	r := New(db, server.Client(), 0)
	for n := range 3 {
		if n == 2 {
			version.Store(2)
		}
		if result := r.Refresh(ctx, feed); result.Error != nil {
			t.Fatal(result.Error)
		}
		items, err := db.Episodes(ctx, feed.ID)
		if err != nil || len(items) != 1 || len(items[0].ParsedChapters()) != 2 || items[0].ChaptersState != "external" || !items[0].HasChapters() {
			t.Fatal("linked chapters not cached on feed refresh", items, err)
		}
		want := int32(1)
		if n == 2 {
			want = 2
			if !strings.HasSuffix(items[0].ChaptersURL, "chapter2.json") {
				t.Fatal("URL not replaced")
			}
		}
		if requested.Load() != want {
			t.Fatalf("refresh %d fetched file %d times, want %d", n, requested.Load(), want)
		}
	}
}

func TestRefreshRemembersEmptyAndBrokenChapterFiles(t *testing.T) {
	var requested atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed":
			fmt.Fprint(w, `<rss version="2.0" xmlns:podcast="http://podcastindex.org/namespace/1.0"><channel><title>Show</title><item><guid>empty</guid><title>Empty</title><enclosure url="https://example.test/empty" type="audio/mpeg"/><podcast:chapters url="/empty.json"/></item><item><guid>error</guid><title>Error</title><enclosure url="https://example.test/error" type="audio/mpeg"/><podcast:chapters url="/error.json"/></item></channel></rss>`)
		case "/empty.json":
			requested.Add(1)
			fmt.Fprint(w, `{"chapters":[]}`)
		case "/error.json":
			requested.Add(1)
			w.WriteHeader(503)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	db := database(t)
	feed := addFeed(t, db, server.URL+"/feed")
	r := New(db, server.Client(), 0)
	for range 2 {
		if result := r.Refresh(ctx, feed); result.Error != nil {
			t.Fatal(result.Error)
		}
	}
	items, err := db.Episodes(ctx, feed.ID)
	if err != nil || len(items) != 2 || requested.Load() != 2 {
		t.Fatal(items, err, requested.Load())
	}
	states := make(map[string]string)
	for _, item := range items {
		states[item.GUID] = item.ChaptersState
	}
	if states["empty"] != "empty" || states["error"] != "error" {
		t.Fatal(states)
	}
	// Both failures stay associated with the URL and must not become loaded timelines.
	for _, item := range items {
		if len(item.ParsedChapters()) != 0 {
			t.Fatal("broken file was cached", item)
		}
	}
}
