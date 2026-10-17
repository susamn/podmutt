package feed

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"podmutt/internal/store"
	"testing"
)

func TestFetchValidatesWithoutPersistence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"preview"`)
		fmt.Fprint(w, rssFixture)
	}))
	defer server.Close()
	db := database(t)
	fetched, err := New(db, server.Client(), 0).Fetch(ctx, store.Feed{URL: server.URL})
	if err != nil || len(fetched.Document.Episodes) != 2 || fetched.ETag != `"preview"` {
		t.Fatalf("fetch: %+v %v", fetched, err)
	}
	if feeds, err := db.Feeds(ctx, false); err != nil || len(feeds) != 0 {
		t.Fatal("fetch persisted data")
	}
}
