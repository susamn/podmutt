package playback

import (
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
	ok, err := db.CacheEmbeddedChapters(background, item.ID, item.AudioURL, encoded)
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
