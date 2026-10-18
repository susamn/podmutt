package library

import (
	"errors"
	"testing"
)

func TestAdhocCachePreservesSubscriptionValidatorsAndHistory(t *testing.T) {
	db := database(t)
	fetcher := &fixtureFetcher{}
	s := New(db, fetcher)
	preview, err := s.Preview(ctx, "https://example.test/feed#fragment")
	if err != nil {
		t.Fatal(err)
	}
	preview.Fetched.Document.Episodes[1].GUID = ""
	ids, err := s.CachePreviewEpisodes(ctx, preview, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	sources, err := db.Feeds(ctx, false)
	if err != nil || len(sources) != 1 || sources[0].Subscribed || sources[0].ETag != "" || sources[0].URL != "https://example.test/feed" {
		t.Fatal("ad-hoc subscribed/advanced validators", sources, err)
	}
	cached, err := db.Episodes(ctx, sources[0].ID)
	if err != nil || len(cached) != 1 || cached[0].ID != ids[0] {
		t.Fatal("stored unselected backlog", cached, err)
	}
	if err := db.Checkpoint(ctx, ids[0], 42); err != nil {
		t.Fatal(err)
	}
	if err := db.SetCompleted(ctx, ids[0], true); err != nil {
		t.Fatal(err)
	}
	again, err := s.CachePreviewEpisodes(ctx, preview, []int{1})
	if err != nil || again[0] != ids[0] {
		t.Fatal("identity changed", again, err)
	}
	history, err := db.Playback(ctx, ids[0])
	if err != nil || history.PositionSeconds != 42 || !history.Completed {
		t.Fatal("history overwritten", history, err)
	}
	id, err := s.SubscribePreview(ctx, preview)
	if err != nil || id != sources[0].ID {
		t.Fatal(err)
	}
	cached, err = db.Episodes(ctx, id)
	if err != nil || len(cached) != 2 {
		t.Fatal("later subscribe missed full preview", cached, err)
	}
	sources, err = db.Feeds(ctx, true)
	if err != nil || len(sources) != 1 || sources[0].ETag != `"v1"` {
		t.Fatal("subscribe validators", sources, err)
	}
	if _, err := s.CachePreviewEpisodes(ctx, preview, []int{0, 1}); err != nil {
		t.Fatal(err)
	}
	sources, err = db.Feeds(ctx, true)
	if err != nil || len(sources) != 1 || sources[0].ETag != `"v1"` || fetcher.calls != 1 {
		t.Fatal("cache changed validators/subscription/refetched", sources, err)
	}
}

func TestAdhocCacheRejectsInvalidSelectionsAndPropagatesStorageErrors(t *testing.T) {
	for _, op := range []string{"feeds", "feed", "episodes", "show"} {
		t.Run(op, func(t *testing.T) {
			db := database(t)
			s := New(&brokenRepo{Repository: db, op: op}, &fixtureFetcher{})
			preview, err := s.Preview(ctx, "https://example.test/feed")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.CachePreviewEpisodes(ctx, preview, []int{0}); !errors.Is(err, failure) {
				t.Fatal("failure hidden", err)
			}
			subscribed, err := db.Feeds(ctx, true)
			if err != nil || len(subscribed) != 0 {
				t.Fatal("failed cache subscribed", subscribed, err)
			}
		})
	}
	db := database(t)
	s := New(db, &fixtureFetcher{})
	preview, err := s.Preview(ctx, "https://example.test/feed")
	if err != nil {
		t.Fatal(err)
	}
	for _, indices := range [][]int{nil, {-1}, {2}} {
		if _, err := s.CachePreviewEpisodes(ctx, preview, indices); err == nil {
			t.Fatal("invalid selection accepted", indices)
		}
	}
	invalid := preview
	invalid.URL = "file:///private"
	if _, err := s.CachePreviewEpisodes(ctx, invalid, []int{0}); err == nil {
		t.Fatal("invalid feed URL")
	}
	invalid = preview
	invalid.Fetched.NotModified = true
	if _, err := s.CachePreviewEpisodes(ctx, invalid, []int{0}); err == nil {
		t.Fatal("empty preview")
	}
	preview.Fetched.Document.Episodes[0].AudioURL = "file:///private"
	if _, err := s.CachePreviewEpisodes(ctx, preview, []int{0}); err == nil {
		t.Fatal("invalid audio")
	}
	sources, err := db.Feeds(ctx, false)
	if err != nil || len(sources) != 0 {
		t.Fatal("invalid input wrote a feed", sources, err)
	}
}
