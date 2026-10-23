package playback

import (
	"testing"

	"podmutt/internal/store"
)

func TestRefreshChaptersUpdatesCurrentAndQueueWithoutChangingTransport(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	ready(t, e, p, 25)
	if err := e.Pause(background); err != nil {
		t.Fatal(err)
	}
	if err := e.Enqueue(background, items[1].ID); err != nil {
		t.Fatal(err)
	}
	before := state(t, e)
	for _, item := range items[:2] {
		item.ChaptersURL = "https://example.test/chapters.json"
		item.ChaptersJSON = `[{"index":1,"start":0,"title":"Opening"}]`
		if err := db.UpsertEpisodes(background, item.FeedID, []store.Episode{item}); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.RefreshChapters(background, items[0].FeedID); err != nil {
		t.Fatal(err)
	}
	after := state(t, e)
	if after.PlaybackID != before.PlaybackID || after.Status != Paused || after.Current.ID != before.Current.ID || after.Current.PositionSeconds != before.Current.PositionSeconds || len(after.Queue) != 1 || after.Queue[0].ID != before.Queue[0].ID || !after.Current.HasChapters() || !after.Queue[0].HasChapters() {
		t.Fatal("refresh changed transport/queue or lost chapter metadata", after)
	}
	// A feed changing an audio URL must not attach the new audio's chapter
	// timeline to the currently loaded old audio.
	replaced := items[0]
	replaced.AudioURL = "https://example.test/new-audio.mp3"
	replaced.ChaptersURL = "https://example.test/new-chapters.json"
	replaced.ChaptersJSON = `[{"index":1,"start":50,"title":"New audio"}]`
	if err := db.UpsertEpisodes(background, replaced.FeedID, []store.Episode{replaced}); err != nil {
		t.Fatal(err)
	}
	if err := e.RefreshChapters(background, replaced.FeedID); err != nil {
		t.Fatal(err)
	}
	if state(t, e).Current.ChaptersURL != "https://example.test/chapters.json" {
		t.Fatal("replacement metadata applied to old audio")
	}
}
