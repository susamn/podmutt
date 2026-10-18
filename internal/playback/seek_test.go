package playback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"podmutt/internal/player"
	"podmutt/internal/store"
)

func TestSeekingClampsPersistsAndPreservesPauseAndQueue(t *testing.T) {
	db, p, items := setup(t)
	item := items[0]
	item.DurationSeconds = 100
	if err := db.UpsertEpisodes(background, item.FeedID, []store.Episode{item}); err != nil {
		t.Fatal(err)
	}
	e, _ := engineFor(t, db, p)
	if err := e.SeekBy(background, 15); !errors.Is(err, ErrSeekUnavailable) {
		t.Fatal(err)
	}
	if err := e.PlayEpisode(background, item.ID); err != nil {
		t.Fatal(err)
	}
	if state(t, e).CanSeek {
		t.Fatal("seek before load advertised")
	}
	if err := e.SeekBy(background, 15); !errors.Is(err, ErrSeekUnavailable) {
		t.Fatal(err)
	}
	ready(t, e, p, 40)
	if !state(t, e).CanSeek {
		t.Fatal("loaded seek unavailable")
	}
	if err := e.Enqueue(background, items[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := e.Pause(background); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct{ delta, want float64 }{{15, 55}, {15, 70}, {100, 100}, {-1000, 0}} {
		if err := e.SeekBy(background, step.delta); err != nil {
			t.Fatal(err)
		}
		s := state(t, e)
		if s.Status != Paused || s.Current.PositionSeconds != step.want || len(s.Queue) != 1 {
			t.Fatal("seek changed pause/queue", s)
		}
		saved, err := db.Playback(background, item.ID)
		if err != nil || saved.PositionSeconds != step.want {
			t.Fatal("seek not checkpointed", saved, err)
		}
	}
	if err := e.SeekTo(background, -20); err != nil || state(t, e).Current.PositionSeconds != 0 {
		t.Fatal("absolute clamp", err)
	}
	if err := e.SeekTo(background, 35); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := e.SeekBy(background, value); err == nil {
			t.Fatal("nonfinite seek")
		}
	}
	if err := e.Stop(background); err != nil {
		t.Fatal(err)
	}
	if state(t, e).CanSeek {
		t.Fatal("stopped seek advertised")
	}
	if err := e.SeekBy(background, 15); !errors.Is(err, ErrSeekUnavailable) {
		t.Fatal(err)
	}
	if err := e.Play(background); err != nil {
		t.Fatal(err)
	}
	if state(t, e).Current.PositionSeconds != 35 {
		t.Fatal("stop/restore lost seek")
	}
}

func TestSeekingRejectsStalePositionsAndBoundsCorrection(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	ready(t, e, p, 40)
	if err := e.SeekBy(background, 15); err != nil {
		t.Fatal(err)
	}
	apply := func(position float64, expired bool) {
		t.Helper()
		_, err := e.submit(background, func(ctx context.Context) error {
			if expired {
				e.seekUntil = time.Now().Add(-time.Second)
			}
			e.event(ctx, player.Event{Name: "property-change", Property: "time-pos", Data: json.RawMessage(fmt.Sprint(position))})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	apply(40, false)
	if state(t, e).Current.PositionSeconds != 55 {
		t.Fatal("stale pre-seek observation undid seek")
	}
	if err := e.SeekBy(background, 15); err != nil {
		t.Fatal(err)
	}
	if state(t, e).Current.PositionSeconds != 70 {
		t.Fatal("rapid seeks did not accumulate")
	}
	apply(70.1, false)
	if state(t, e).Current.PositionSeconds != 70.1 {
		t.Fatal("actual position ignored")
	}
	if err := e.SeekTo(background, 10000); err != nil {
		t.Fatal("unknown duration seek", err)
	}
	apply(80, true)
	if state(t, e).Current.PositionSeconds != 80 {
		t.Fatal("correction window never expired")
	}
	p.setFailure("seek")
	if err := e.SeekBy(background, 15); err == nil {
		t.Fatal("device rejection hidden")
	}
	s := state(t, e)
	if s.Status != Playing || s.Current.PositionSeconds != 80 || !strings.Contains(s.Error, "Seek failed") {
		t.Fatal("rejected seek stopped/reset playback", s)
	}
	p.setFailure("")
	if err := e.SeekBy(background, -15); err != nil {
		t.Fatal(err)
	}
	if state(t, e).Error != "" {
		t.Fatal("seek error not cleared")
	}
}

type nonSeekingPlayer struct{ Player }

func TestUnsupportedSeekingAndEOFAreSafe(t *testing.T) {
	db, p, items := setup(t)
	e, err := New(background, db, func(context.Context) (Player, error) { return &nonSeekingPlayer{p}, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close(background)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	ready(t, e, p, 12)
	if state(t, e).CanSeek {
		t.Fatal("unsupported seeking advertised")
	}
	if err := e.SeekTo(background, 15); !errors.Is(err, ErrSeekUnavailable) {
		t.Fatal(err)
	}
	e.Close(background)
	e, _ = engineFor(t, db, p)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	id := ready(t, e, p, 12)
	if err := e.Enqueue(background, items[1].ID); err != nil {
		t.Fatal(err)
	}
	p.event("end-file", "eof", id)
	if err := e.SeekBy(background, 15); err == nil {
		t.Fatal("seek applied across EOF/load boundary")
	}
	eventually(t, func() bool { return state(t, e).Current.ID == items[1].ID })
	p.mu.Lock()
	count := len(p.seeks)
	p.mu.Unlock()
	if count != 0 {
		t.Fatal("seek reached next episode")
	}
}
