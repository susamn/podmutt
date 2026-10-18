package playback

import (
	"encoding/json"
	"podmutt/internal/player"
	"testing"
)

func TestPositionAnchorSurvivesQueueOnlyUpdates(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	if !state(t, e).ClockAt.IsZero() {
		t.Fatal("loading clock advanced without readiness")
	}
	ready(t, e, p, 42)
	anchor := state(t, e).ClockAt
	for _, value := range []string{"6.25", "null", "-1", "0", "1e100", "\"invalid\""} {
		p.events <- player.Event{Name: "property-change", Property: "duration", Data: json.RawMessage(value)}
	}
	eventually(t, func() bool { return state(t, e).Current.DurationSeconds == 7 })
	if state(t, e).ClockAt != anchor {
		t.Fatal("duration update reset position anchor")
	}
	if anchor.IsZero() {
		t.Fatal("moving position has no anchor")
	}
	if err := e.Enqueue(background, items[1].ID); err != nil {
		t.Fatal(err)
	}
	if state(t, e).ClockAt != anchor {
		t.Fatal("queue publication reset old position anchor")
	}
	if err := e.Pause(background); err != nil {
		t.Fatal(err)
	}
	if !state(t, e).ClockAt.IsZero() {
		t.Fatal("paused clock still advances")
	}
	if err := e.Play(background); err != nil {
		t.Fatal(err)
	}
	if state(t, e).ClockAt.IsZero() {
		t.Fatal("resume did not anchor position")
	}
	if err := e.Stop(background); err != nil {
		t.Fatal(err)
	}
	if !state(t, e).ClockAt.IsZero() {
		t.Fatal("stopped clock still advances")
	}
}
