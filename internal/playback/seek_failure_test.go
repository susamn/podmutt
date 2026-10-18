package playback

import "testing"

func TestSeekPersistenceFailureStopsWithoutConsumingQueue(t *testing.T) {
	db, p, items := setup(t)
	repo := &faultRepository{Repository: db}
	e, _ := engineFor(t, repo, p)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	ready(t, e, p, 20)
	if err := e.Enqueue(background, items[1].ID); err != nil {
		t.Fatal(err)
	}
	repo.set("checkpoint")
	if err := e.SeekBy(background, 15); err == nil {
		t.Fatal("checkpoint failure hidden")
	}
	s := state(t, e)
	if s.Status != Failed || len(s.Queue) != 1 || s.Queue[0].ID != items[1].ID {
		t.Fatal("seek failure lost work", s)
	}
	repo.set("")
}
func TestSeekPositionFailureDoesNotInventPosition(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	ready(t, e, p, 20)
	p.setFailure("position")
	if err := e.SeekBy(background, 15); err == nil {
		t.Fatal("position failure ignored")
	}
	if state(t, e).Current.PositionSeconds != 20 {
		t.Fatal("invented seek position")
	}
	p.setFailure("")
}
