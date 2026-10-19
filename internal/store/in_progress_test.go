package store

import "testing"

func TestInProgressIncludesUnsubscribedHistoryButNotCompletedOrUnplayed(t *testing.T) {
	s, _ := openStore(t)
	subscribed := addEpisodes(t, s, addFeed(t, s, "subscribed", true))
	preview := addEpisodes(t, s, addFeed(t, s, "preview", false))
	for _, episode := range []Episode{subscribed[0], preview[0], preview[1]} {
		if err := s.Checkpoint(ctx, episode.ID, 42); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetCompleted(ctx, preview[1].ID, true); err != nil {
		t.Fatal(err)
	}
	rows, err := s.InProgress(ctx)
	if err != nil || len(rows) != 2 || rows[0].ID != preview[0].ID || rows[1].ID != subscribed[0].ID {
		t.Fatal("unfinished history missing or unplayed/completed leaked", rows, err)
	}
	if rows[0].PositionSeconds != 42 || rows[0].FeedTitle != "preview" {
		t.Fatal("resume metadata missing", rows[0])
	}
	s.Close()
	if _, err := s.InProgress(ctx); err == nil {
		t.Fatal("closed database accepted")
	}
}
