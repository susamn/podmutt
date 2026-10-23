package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverSnapshotsPreserveOrderedViewAndBoundRetention(t *testing.T) {
	s, _ := openStore(t)
	ctx := context.Background()
	results, _ := json.Marshal([]map[string]string{{"title": "Rank 1"}, {"title": "Rank 2"}})
	fetched := time.Unix(100, 0).UTC()
	if err := s.SaveDiscoverSnapshot(ctx, DiscoverSnapshot{Provider: "APPLE", Country: "US", Mode: "chart", Results: results, FetchedAt: fetched}); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadDiscoverSnapshot(ctx, "apple", "us", "", "chart")
	if err != nil || string(got.Results) != string(results) || !got.FetchedAt.Equal(fetched) {
		t.Fatal(got, err)
	}
	if _, err := s.LoadDiscoverSnapshot(ctx, "apple", "de", "", "chart"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("country keys collided", err)
	}
	if err := s.SaveDiscoverSnapshot(ctx, DiscoverSnapshot{Provider: "podcast_index", Country: "us", Query: "one", Mode: "search", Results: results}); err == nil {
		t.Fatal("persisted provider-restricted Index response")
	}
	if err := s.SaveDiscoverSnapshot(ctx, DiscoverSnapshot{Provider: "apple", Country: "us", Query: "  science   news ", Mode: "search", Results: results}); err != nil {
		t.Fatal(err)
	}
	got, err = s.LoadDiscoverSnapshot(ctx, "apple", "US", "science news", "search")
	if err != nil || got.Query != "science news" {
		t.Fatal(got, err)
	}
	if err := s.SaveDiscoverSnapshot(ctx, DiscoverSnapshot{Provider: "apple", Mode: "chart", Results: []byte("[]")}); err != nil {
		t.Fatal("successful empty response not cacheable", err)
	}
	for i := 0; i < 30; i++ {
		if err := s.SaveDiscoverSnapshot(ctx, DiscoverSnapshot{Provider: "apple", Country: "us", Query: fmt.Sprintf("query-%02d", i), Mode: "search", Results: results, FetchedAt: time.Unix(int64(i+1000), 0)}); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM discover_snapshots").Scan(&count); err != nil || count > 24 {
		t.Fatal("retention unbounded", count, err)
	}
}

func TestDiscoverViewPersistsProviderAndSelection(t *testing.T) {
	s, _ := openStore(t)
	ctx := context.Background()
	view := DiscoverView{Provider: "podcast_index", Query: "  Radio   Lab ", Mode: "search", SelectedFeedURL: "https://example.test/rss"}
	if err := s.SaveDiscoverView(ctx, view); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadDiscoverView(ctx)
	if err != nil || got.Provider != "podcast_index" || got.Query != "Radio Lab" || got.SelectedFeedURL != "" {
		t.Fatal(got, err)
	}
	if err := s.SaveDiscoverView(ctx, DiscoverView{Provider: "apple", Country: "DE", Mode: "chart"}); err != nil {
		t.Fatal(err)
	}
	got, err = s.LoadDiscoverView(ctx)
	if err != nil || got.Country != "de" || got.Provider != "apple" || got.Query != "" {
		t.Fatal(got, err)
	}
}

func TestDiscoverPersistenceRejectsInvalidOrOversizedInputs(t *testing.T) {
	s, _ := openStore(t)
	ctx := context.Background()
	for _, snapshot := range []DiscoverSnapshot{
		{Provider: "other", Mode: "chart", Results: []byte("[]")},
		{Provider: "apple", Mode: "unknown", Results: []byte("[]")},
		{Provider: "apple", Mode: "search", Results: []byte("[]")},
		{Provider: "apple", Mode: "chart", Results: nil},
		{Provider: "apple", Mode: "chart", Results: make([]byte, (1<<20)+1)},
	} {
		if err := s.SaveDiscoverSnapshot(ctx, snapshot); err == nil {
			t.Fatalf("accepted invalid snapshot: %+v", snapshot)
		}
	}
	for _, view := range []DiscoverView{
		{Provider: "other", Mode: "chart"},
		{Provider: "apple", Mode: "unknown"},
		{Provider: "apple", Mode: "search"},
		{Provider: "apple", Mode: "chart", SelectedFeedURL: string(make([]byte, 2049))},
	} {
		if err := s.SaveDiscoverView(ctx, view); err == nil {
			t.Fatalf("accepted invalid view: %+v", view)
		}
	}
	if err := s.SaveDiscoverView(ctx, DiscoverView{Provider: "podcast_index", Mode: "search", Query: "query"}); err != nil {
		t.Fatal("Index navigation should allow an empty country", err)
	}
}

func TestDiscoverMigrationRemovesDurablePodcastIndexDirectoryRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrate(context.Background(), db, migrations[:6]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO discovered_shows(title,feed_url,source_id) VALUES('Old Index','https://example.test/index',2)`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(context.Background(), db, migrations); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM discovered_shows WHERE source_id=2").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}
