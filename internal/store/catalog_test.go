package store

import (
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"
)

func addFeed(t *testing.T, s *Store, name string, subscribed bool) int64 {
	t.Helper()
	id, err := s.UpsertFeed(ctx, Feed{URL: "https://example.test/" + name, Title: name, Subscribed: subscribed})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func addEpisodes(t *testing.T, s *Store, feedID int64) []Episode {
	t.Helper()
	items := []Episode{
		{GUID: "old", Title: "Old", AudioURL: "https://example.test/old.mp3", PublishedAt: time.Unix(100, 0).UTC()},
		{GUID: "new", Title: "New", AudioURL: "https://example.test/new.mp3", PublishedAt: time.Unix(200, 0).UTC(), DurationSeconds: 1800},
	}
	if err := s.UpsertEpisodes(ctx, feedID, items); err != nil {
		t.Fatal(err)
	}
	got, err := s.Episodes(ctx, feedID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestFeedMetadataAndSubscription(t *testing.T) {
	s, _ := openStore(t)
	z := addFeed(t, s, "Z", true)
	a := addFeed(t, s, "a", false)
	updated := Feed{URL: "https://example.test/Z", Title: "Z updated", ArtworkURL: "https://example.test/art.png", ETag: "etag", LastModified: "date"}
	id, err := s.UpsertFeed(ctx, updated)
	if err != nil || id != z {
		t.Fatalf("update changed feed identity: %d, %v", id, err)
	}
	feeds, err := s.Feeds(ctx, true)
	if err != nil || len(feeds) != 1 {
		t.Fatalf("subscribed feeds = %v, %v", feeds, err)
	}
	updated.ID, updated.Subscribed = z, true
	if !reflect.DeepEqual(feeds[0], updated) {
		t.Fatalf("metadata = %+v, want %+v", feeds[0], updated)
	}
	if err := s.SetSubscribed(ctx, a, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSubscribed(ctx, z, false); err != nil {
		t.Fatal(err)
	}
	feeds, err = s.Feeds(ctx, false)
	if err != nil || len(feeds) != 2 || feeds[0].ID != a || feeds[1].Subscribed {
		t.Fatalf("all feeds = %v, %v", feeds, err)
	}
	if err := s.SetSubscribed(ctx, 999, true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing feed: %v", err)
	}
	for _, raw := range []string{"", "ftp://example.test", "https://user:secret@example.test/feed", "https://", "http://["} {
		if _, err := s.UpsertFeed(ctx, Feed{URL: raw}); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}

func TestEpisodeIdentityAndRefresh(t *testing.T) {
	s, _ := openStore(t)
	feed := addFeed(t, s, "feed", true)
	original := addEpisodes(t, s, feed)
	newest := original[0]
	newest.AudioURL = "https://example.test/changed-enclosure.mp3"
	newest.Title, newest.Description, newest.ArtworkURL = "Updated", "Details", "https://example.test/art.png"
	if err := s.UpsertEpisodes(ctx, feed, []Episode{newest}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Episodes(ctx, feed)
	if err != nil || len(got) != 2 || !reflect.DeepEqual(got[0], newest) {
		t.Fatalf("refresh = %+v, %v", got, err)
	}
	other := addFeed(t, s, "preview", false)
	addEpisodes(t, s, other) // GUIDs are scoped to a feed.
	all, err := s.Episodes(ctx, 0)
	if err != nil || len(all) != 2 {
		t.Fatalf("subscribed aggregate = %+v, %v", all, err)
	}
	if err := s.SetSubscribed(ctx, other, true); err != nil {
		t.Fatal(err)
	}
	all, err = s.Episodes(ctx, 0)
	if err != nil || len(all) != 4 || all[0].PublishedAt.Before(all[1].PublishedAt) {
		t.Fatalf("aggregate = %+v, %v", all, err)
	}
	fallback := Episode{AudioURL: "https://example.test/no-guid.mp3"}
	for range 2 {
		if err := s.UpsertEpisodes(ctx, feed, []Episode{fallback}); err != nil {
			t.Fatal(err)
		}
	}
	got, err = s.Episodes(ctx, feed)
	if err != nil || len(got) != 3 || !got[2].PublishedAt.IsZero() {
		t.Fatalf("fallback identity = %+v, %v", got, err)
	}
}

func TestEpisodeBatchRollsBack(t *testing.T) {
	s, _ := openStore(t)
	feed := addFeed(t, s, "feed", true)
	valid := Episode{GUID: "valid", AudioURL: "https://example.test/a.mp3"}
	for _, invalid := range []Episode{
		{GUID: "bad-url", AudioURL: "file:///tmp/audio.mp3"},
		{GUID: "bad-duration", AudioURL: valid.AudioURL, DurationSeconds: -1},
	} {
		if err := s.UpsertEpisodes(ctx, feed, []Episode{valid, invalid}); err == nil {
			t.Fatal("invalid batch accepted")
		}
		got, err := s.Episodes(ctx, feed)
		if err != nil || len(got) != 0 {
			t.Fatalf("partial batch persisted: %v, %v", got, err)
		}
	}
	if err := s.UpsertEpisodes(ctx, 999, []Episode{valid}); err == nil {
		t.Fatal("orphan episode accepted")
	}
}

func TestCatalogClosedDatabase(t *testing.T) {
	s, _ := openStore(t)
	s.Close()
	if _, err := s.UpsertFeed(ctx, Feed{URL: "https://example.test"}); err == nil {
		t.Fatal("upsert on closed database succeeded")
	}
	if _, err := s.Feeds(ctx, false); err == nil {
		t.Fatal("list on closed database succeeded")
	}
	if err := s.SetSubscribed(ctx, 1, true); err == nil {
		t.Fatal("subscription update on closed database succeeded")
	}
	if err := s.UpsertEpisodes(ctx, 1, nil); err == nil {
		t.Fatal("batch on closed database succeeded")
	}
	if _, err := s.Episodes(ctx, 0); err == nil {
		t.Fatal("episodes on closed database succeeded")
	}
}
