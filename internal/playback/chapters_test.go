package playback

import (
	"errors"
	"testing"

	"podmutt/internal/chapters"
	"podmutt/internal/store"
)

func TestEmbeddedChapterCacheSurvivesRefreshButNotAudioReplacement(t *testing.T) {
	db, _, items := setup(t)
	item := items[0]
	encoded, err := chapters.Serialize([]chapters.Chapter{{Index: 1, Start: 0, Title: "Embedded"}})
	if err != nil {
		t.Fatal(err)
	}
	ok, err := db.CacheEmbeddedChapters(background, item.ID, encoded)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err := db.UpsertEpisodes(background, item.FeedID, []store.Episode{item}); err != nil {
		t.Fatal(err)
	}
	got, err := db.Episode(background, item.ID)
	if err != nil || got.ChaptersJSON != encoded {
		t.Fatal("embedded metadata lost on feed refresh", got, err)
	}
	item.AudioURL = "https://example.test/different.mp3"
	if err := db.UpsertEpisodes(background, item.FeedID, []store.Episode{item}); err != nil {
		t.Fatal(err)
	}
	got, err = db.Episode(background, item.ID)
	if err != nil || got.ChaptersJSON != "" {
		t.Fatal("embedded metadata survived audio replacement", got, err)
	}
}

func TestCacheChaptersUsesActiveIdentityAndCurrentURL(t *testing.T) {
	db, p, items := setup(t)
	item := items[0]
	item.ChaptersURL = "https://example.test/chapters.json"
	if err := db.UpsertEpisodes(background, item.FeedID, []store.Episode{item}); err != nil {
		t.Fatal(err)
	}
	e, _ := engineFor(t, db, p)
	list := []chapters.Chapter{{Index: 1, Start: 0, Title: "Intro"}}
	if err := e.CacheChapters(background, item.ID, 0, item.ChaptersURL, list); !errors.Is(err, ErrSeekUnavailable) {
		t.Fatal(err)
	}
	if err := e.PlayEpisode(background, item.ID); err != nil {
		t.Fatal(err)
	}
	ready(t, e, p, 5)
	s := state(t, e)
	if err := e.CacheChapters(background, items[1].ID, s.PlaybackID, item.ChaptersURL, list); !errors.Is(err, ErrSeekUnavailable) {
		t.Fatal(err)
	}
	if err := e.CacheChapters(background, item.ID, s.PlaybackID, "https://example.test/wrong.json", list); !errors.Is(err, ErrSeekUnavailable) {
		t.Fatal(err)
	}
	if err := e.CacheChapters(background, item.ID, s.PlaybackID, item.ChaptersURL, list); err != nil {
		t.Fatal(err)
	}
	cached, err := db.Episode(background, item.ID)
	if err != nil || len(cached.ParsedChapters()) != 1 || len(state(t, e).Current.ParsedChapters()) != 1 {
		t.Fatal("actor/store cache disagreed", cached, err)
	}
	// Feed refresh changes the declaration before the asynchronous response.
	item.ChaptersURL = "https://example.test/new.json"
	item.ChaptersJSON = ""
	if err := db.UpsertEpisodes(background, item.FeedID, []store.Episode{item}); err != nil {
		t.Fatal(err)
	}
	if err := e.CacheChapters(background, item.ID, s.PlaybackID, "https://example.test/chapters.json", list); !errors.Is(err, ErrSeekUnavailable) {
		t.Fatal("stale URL saved", err)
	}
	cached, err = db.Episode(background, item.ID)
	if err != nil || cached.ChaptersJSON != "" {
		t.Fatal("refresh retained old cache", cached, err)
	}
	if err := e.Stop(background); err != nil {
		t.Fatal(err)
	}
	if err := e.CacheChapters(background, item.ID, s.PlaybackID, item.ChaptersURL, list); !errors.Is(err, ErrSeekUnavailable) {
		t.Fatal("stopped player accepted metadata", err)
	}
}
