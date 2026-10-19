package playback

import (
	"testing"
	"time"

	"podmutt/internal/store"
)

func TestNewestPendingPriorityDoesNotChangeQueueOrder(t *testing.T) {
	for _, test := range []struct {
		name  string
		queue []store.Episode
		want  int64
	}{
		{name: "empty"},
		{name: "newest not at head", queue: []store.Episode{{ID: 1, PublishedAt: time.Unix(100, 0)}, {ID: 2, PublishedAt: time.Unix(300, 0)}, {ID: 3, PublishedAt: time.Unix(200, 0)}}, want: 2},
		{name: "unknown last", queue: []store.Episode{{ID: 1}, {ID: 2, PublishedAt: time.Unix(100, 0)}}, want: 2},
		{name: "unknown stable", queue: []store.Episode{{ID: 3}, {ID: 1}, {ID: 2}}, want: 3},
		{name: "equal stable", queue: []store.Episode{{ID: 2, PublishedAt: time.Unix(100, 0)}, {ID: 1, PublishedAt: time.Unix(100, 0)}}, want: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			e := &Engine{state: State{Queue: test.queue}}
			before := queueIDs(e.state.Queue)
			if got := e.nextPendingID(); got != test.want {
				t.Fatal(got, test.want)
			}
			for i, id := range before {
				if e.state.Queue[i].ID != id {
					t.Fatal("priority sorted the queue")
				}
			}
		})
	}
}

func TestManualPlaybackEOFPicksNewestNotNextInserted(t *testing.T) {
	db, p, _ := setup(t)
	feeds, err := db.Feeds(background, true)
	if err != nil {
		t.Fatal(err)
	}
	feedID := feeds[0].ID
	if err := db.UpsertEpisodes(background, feedID, []store.Episode{
		{GUID: "a", AudioURL: "https://example.test/a.mp3", PublishedAt: time.Unix(100, 0)},
		{GUID: "b", AudioURL: "https://example.test/b.mp3", PublishedAt: time.Unix(300, 0)},
		{GUID: "c", AudioURL: "https://example.test/c.mp3", PublishedAt: time.Unix(200, 0)},
		{GUID: "manual", AudioURL: "https://example.test/manual.mp3", PublishedAt: time.Unix(50, 0)},
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Episodes(background, feedID)
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]int64)
	for _, episode := range rows {
		ids[episode.GUID] = episode.ID
	}
	e, _ := engineFor(t, db, p)
	// The manual/tail order is deliberately not chronological.
	if err := e.Enqueue(background, ids["a"], ids["b"], ids["c"], ids["manual"]); err != nil {
		t.Fatal(err)
	}
	if err := e.PlayEpisode(background, ids["manual"]); err != nil {
		t.Fatal(err)
	}
	before := state(t, e)
	if len(before.Queue) != 3 || before.Queue[0].ID != ids["a"] || before.Queue[1].ID != ids["b"] {
		t.Fatal("insertion order changed", before)
	}
	entry := ready(t, e, p, 12)
	p.event("end-file", "eof", entry)
	eventually(t, func() bool { return state(t, e).Current.ID == ids["b"] })
	if history, err := db.Playback(background, ids["manual"]); err != nil || !history.Completed {
		t.Fatal("manual episode was not completed", history, err)
	}
	// Next uses the same priority, but must not mark the skipped episode complete.
	if err := e.Next(background); err != nil {
		t.Fatal(err)
	}
	if got := state(t, e); got.Current.ID != ids["c"] || len(got.Queue) != 1 || got.Queue[0].ID != ids["a"] {
		t.Fatal("Next did not use newest priority", got)
	}
	if history, err := db.Playback(background, ids["b"]); err != nil || history.Completed {
		t.Fatal("Next marked completion", history, err)
	}
}
