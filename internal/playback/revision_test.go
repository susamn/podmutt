package playback

import (
	"testing"

	"podmutt/internal/player"
)

func TestPublicationRevisionsMatchRepliesAndDoNotAdvanceOnReads(t *testing.T) {
	repo, p, episodes := setup(t)
	e, _ := engineFor(t, repo, p)
	initial := state(t, e)
	if initial.Revision == 0 || state(t, e).Revision != initial.Revision {
		t.Fatal("reads changed publication identity")
	}
	if err := e.Enqueue(background, episodes[1].ID); err != nil {
		t.Fatal(err)
	}
	enqueued := state(t, e)
	publication := <-e.Updates()
	if enqueued.Revision <= initial.Revision || publication.Revision != enqueued.Revision {
		t.Fatal("enqueue reply and publication are not correlated", enqueued.Revision, publication.Revision)
	}
	if err := e.PlayEpisode(background, episodes[0].ID); err != nil {
		t.Fatal(err)
	}
	playing := state(t, e)
	if playing.Revision <= enqueued.Revision {
		t.Fatal("activation did not advance revision")
	}
	entry, _ := p.EntryID(background)
	p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
	p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: []byte("5")}
	eventually(t, func() bool { return state(t, e).Current.PositionSeconds == 5 })
	if sampled := state(t, e); sampled.Revision <= playing.Revision || len(sampled.Queue) != 1 {
		t.Fatal("progress publication changed queue or lost revision", sampled)
	}
}
