package playback

import "testing"

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
