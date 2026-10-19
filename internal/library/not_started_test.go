package library

import (
	"reflect"
	"testing"

	"podmutt/internal/store"
)

func TestLiveProgressSeparatesUnstartedAndContinueWithoutMutatingCache(t *testing.T) {
	unstarted := store.Episode{ID: 1}
	started := store.Episode{ID: 2, PositionSeconds: 12}
	snapshot := Snapshot{All: []store.Episode{unstarted, started}, Show: []store.Episode{unstarted}, New: []store.Episode{unstarted}, Continue: []store.Episode{started}}
	if got := snapshot.WithCurrent(nil); !reflect.DeepEqual(got, snapshot) {
		t.Fatal("nil current changed snapshot")
	}
	for _, current := range []store.Episode{
		{ID: 1, PositionSeconds: .1}, {ID: 1, Completed: true}, {ID: 2},
		{ID: 3, PositionSeconds: 42}, // Unsubscribed explicit playback.
	} {
		got := snapshot.WithCurrent(&current)
		for _, episode := range got.New {
			if episode.Completed || episode.PositionSeconds != 0 {
				t.Fatal("Not started contains history", got)
			}
			for _, continuing := range got.Continue {
				if continuing.ID == episode.ID {
					t.Fatal("Not started and Continue overlap", got)
				}
			}
		}
		if len(got.All) != 2 || got.All[0].ID != 1 || got.All[1].ID != 2 {
			t.Fatal("live playback altered subscribed scope", got)
		}
		if current.ID == 3 && (len(got.Continue) != 2 || got.Continue[1].ID != 3) {
			t.Fatal("unsubscribed progress missing", got)
		}
	}
	if snapshot.All[0].PositionSeconds != 0 || snapshot.Show[0].Completed || len(snapshot.New) != 1 || snapshot.Continue[0].ID != 2 {
		t.Fatal("overlay mutated cached snapshot", snapshot)
	}
	if NewView.String() != "Not started" {
		t.Fatal("ambiguous view label")
	}
}
