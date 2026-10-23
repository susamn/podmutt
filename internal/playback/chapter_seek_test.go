package playback

import (
	"errors"
	"testing"
)

func TestChapterSeekGuardsPlaybackIdentityAndPreservesState(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	ready(t, e, p, 12)
	s := state(t, e)
	if s.PlaybackID == 0 || !s.CanSeek {
		t.Fatal("missing active playback identity", s)
	}
	if err := e.Pause(background); err != nil {
		t.Fatal(err)
	}
	if err := e.SeekChapter(background, s.Current.ID, s.PlaybackID, 90); err != nil {
		t.Fatal(err)
	}
	s = state(t, e)
	if s.Status != Paused || s.Current.PositionSeconds != 90 {
		t.Fatal("chapter seek changed transport or missed position", s)
	}
	progress, err := db.Playback(background, items[0].ID)
	if err != nil || progress.PositionSeconds != 90 {
		t.Fatal("chapter seek did not checkpoint", progress, err)
	}
	oldToken := s.PlaybackID
	if err := e.PlayEpisode(background, items[1].ID); err != nil {
		t.Fatal(err)
	}
	ready(t, e, p, 5)
	if err := e.SeekChapter(background, items[0].ID, oldToken, 300); !errors.Is(err, ErrSeekUnavailable) {
		t.Fatal("stale episode chapter seek accepted", err)
	}
	current := state(t, e)
	if current.Current.ID != items[1].ID || current.Current.PositionSeconds != 5 {
		t.Fatal("stale seek mutated next episode", current)
	}
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	ready(t, e, p, 10)
	current = state(t, e)
	if current.PlaybackID == oldToken {
		t.Fatal("reloaded same episode reused old identity")
	}
	if err := e.SeekChapter(background, items[0].ID, oldToken, 300); !errors.Is(err, ErrSeekUnavailable) {
		t.Fatal("old session token accepted for same episode", err)
	}
	if err := e.Stop(background); err != nil {
		t.Fatal(err)
	}
	if err := e.SeekChapter(background, items[0].ID, current.PlaybackID, 300); !errors.Is(err, ErrSeekUnavailable) {
		t.Fatal("stopped playback unexpectedly seeked", err)
	}
	if err := e.SeekChapter(background, 0, 0, 1); !errors.Is(err, ErrSeekUnavailable) {
		t.Fatal(err)
	}
	if saved, err := db.Episode(background, items[1].ID); err != nil || saved.ID == 0 || saved.Completed {
		t.Fatal("chapter seeking affected other episode", saved, err)
	}
}
