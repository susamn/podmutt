package playback

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPlaybackStateNeverPublishesRawFactoryErrors(t *testing.T) {
	db, _, items := setup(t)
	failure := errors.New("https://user:private-secret@example.test/audio?token=private-secret")
	e, err := New(background, db, func(context.Context) (Player, error) { return nil, failure })
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close(background)
	if err := e.PlayEpisode(background, items[0].ID); !errors.Is(err, failure) {
		t.Fatal("internal cause lost", err)
	}
	snapshot := state(t, e)
	if snapshot.Status != Failed || snapshot.ErrorRevision == 0 || !strings.Contains(snapshot.Error, "Playback failed") || strings.Contains(snapshot.Error, "private-secret") {
		t.Fatal(snapshot)
	}
	if err := e.PlayEpisode(background, items[0].ID); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if state(t, e).ErrorRevision <= snapshot.ErrorRevision {
		t.Fatal("new identical failure was not identified")
	}

}
