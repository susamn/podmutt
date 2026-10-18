package playback

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"podmutt/internal/player"
	"podmutt/internal/store"
)

var background = context.Background()
var testFailure = errors.New("test failure")

type fakePlayer struct {
	mu       sync.Mutex
	events   chan player.Event
	entry    int64
	position float64
	starts   []float64
	seeks    []float64
	loads    int
	stops    int
	closed   int
	failure  string
}

func (p *fakePlayer) op(name string) error {
	if p.failure == name {
		return testFailure
	}
	return nil
}
func (p *fakePlayer) Load(_ context.Context, _ string, position float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.op("load"); err != nil {
		return err
	}
	p.entry++
	p.loads++
	p.position = position
	p.starts = append(p.starts, position)
	return nil
}
func (p *fakePlayer) EntryID(context.Context) (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.entry, p.op("entry")
}
func (p *fakePlayer) Play(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.op("play")
}
func (p *fakePlayer) Pause(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.op("pause")
}
func (p *fakePlayer) Stop(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stops++
	return p.op("stop")
}
func (p *fakePlayer) Position(context.Context) (float64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.position, p.op("position")
}
func (p *fakePlayer) SeekTo(_ context.Context, position float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.op("seek"); err != nil {
		return err
	}
	p.position = position
	p.seeks = append(p.seeks, position)
	return nil
}
func (p *fakePlayer) Events() <-chan player.Event { return p.events }
func (p *fakePlayer) Err() error                  { return player.ErrDisconnected }
func (p *fakePlayer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed++
	return nil
}
func (p *fakePlayer) setFailure(op string)         { p.mu.Lock(); p.failure = op; p.mu.Unlock() }
func (p *fakePlayer) setPosition(position float64) { p.mu.Lock(); p.position = position; p.mu.Unlock() }
func (p *fakePlayer) event(name, reason string, id int64) {
	p.events <- player.Event{Name: name, Reason: reason, PlaylistEntryID: &id}
}

func setup(t *testing.T) (*store.Store, *fakePlayer, []store.Episode) {
	t.Helper()
	db, err := store.Open(background, filepath.Join(t.TempDir(), "podcasts.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	feed, err := db.UpsertFeed(background, store.Feed{URL: "https://example.test/feed", Subscribed: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertEpisodes(background, feed, []store.Episode{
		{GUID: "a", AudioURL: "https://example.test/a.mp3"},
		{GUID: "b", AudioURL: "https://example.test/b.mp3"},
		{GUID: "c", AudioURL: "https://example.test/c.mp3"},
	}); err != nil {
		t.Fatal(err)
	}
	items, err := db.Episodes(background, feed)
	if err != nil {
		t.Fatal(err)
	}
	return db, &fakePlayer{events: make(chan player.Event, 128)}, items
}

func engineFor(t *testing.T, repo Repository, p *fakePlayer) (*Engine, chan time.Time) {
	t.Helper()
	ticks := make(chan time.Time, 1)
	e, err := newEngine(background, repo, func(context.Context) (Player, error) { return p, nil }, ticks, func() {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close(background) })
	return e, ticks
}

func state(t *testing.T, e *Engine) State {
	t.Helper()
	snapshot, err := e.Snapshot(background)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func eventually(t *testing.T, predicate func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if predicate() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("state did not converge")
}

func ready(t *testing.T, e *Engine, p *fakePlayer, position float64) int64 {
	t.Helper()
	id, _ := p.EntryID(background)
	p.setPosition(position)
	p.event("start-file", "", id)
	p.event("file-loaded", "", id)
	data, _ := json.Marshal(position)
	p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: data}
	eventually(t, func() bool { return state(t, e).Current.PositionSeconds == position })
	return id
}

func TestRestoreDoesNotStartPlayer(t *testing.T) {
	db, p, items := setup(t)
	if err := db.Checkpoint(background, items[0].ID, 42); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceQueue(background, []int64{items[1].ID}); err != nil {
		t.Fatal(err)
	}
	e, _ := engineFor(t, db, p)
	snapshot := state(t, e)
	if snapshot.Status != Stopped || snapshot.Current.PositionSeconds != 42 || len(snapshot.Queue) != 1 || p.loads != 0 {
		t.Fatalf("restore = %+v", snapshot)
	}
	// Returned state cannot mutate internal slices or episode fields.
	snapshot.Current.Title = "tampered"
	snapshot.Queue[0].ID = 999
	if got := state(t, e); got.Current.Title == "tampered" || got.Queue[0].ID == 999 {
		t.Fatal("snapshot aliases engine")
	}
	if err := e.Play(background); err != nil {
		t.Fatal(err)
	}
	if p.starts[0] != 42 {
		t.Fatal("resume position lost")
	}
}

func TestCheckpointPauseStopAndClose(t *testing.T) {
	db, p, items := setup(t)
	e, ticks := engineFor(t, db, p)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	ready(t, e, p, 12.5)
	ticks <- time.Now()
	eventually(t, func() bool { saved, _ := db.Playback(background, items[0].ID); return saved.PositionSeconds == 12.5 })
	if err := e.Pause(background); err != nil || state(t, e).Status != Paused {
		t.Fatalf("pause: %v", err)
	}
	if err := e.Play(background); err != nil || state(t, e).Status != Playing {
		t.Fatalf("play: %v", err)
	}
	p.setPosition(20)
	if err := e.Stop(background); err != nil {
		t.Fatal(err)
	}
	saved, _ := db.Playback(background, items[0].ID)
	if saved.PositionSeconds != 20 || saved.Completed || state(t, e).Status != Stopped {
		t.Fatalf("stopped = %+v", saved)
	}
	if err := e.Play(background); err != nil {
		t.Fatal(err)
	}
	ready(t, e, p, 25)
	if err := e.Close(background); err != nil {
		t.Fatal(err)
	}
	saved, _ = db.Playback(background, items[0].ID)
	if saved.PositionSeconds != 25 {
		t.Fatal("exit lost final progress")
	}
	if _, err := e.Snapshot(background); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed request: %v", err)
	}
}

func TestExplicitQueueAndNaturalCompletion(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	a, b, c := items[0].ID, items[1].ID, items[2].ID
	if err := e.Enqueue(background, a, b, b, c); err != nil {
		t.Fatal(err)
	}
	if err := e.SetQueue(background, []int64{c, a, b, c}); err != nil {
		t.Fatal(err)
	}
	if err := e.PlayEpisode(background, a); err != nil {
		t.Fatal(err)
	}
	if got := state(t, e).Queue; len(got) != 2 || got[0].ID != c || got[1].ID != b {
		t.Fatalf("direct play changed remaining queue: %+v", got)
	}
	entry := ready(t, e, p, 30)
	p.event("end-file", "eof", entry)
	eventually(t, func() bool { return state(t, e).Current.ID == c })
	saved, _ := db.Playback(background, a)
	if !saved.Completed {
		t.Fatal("EOF did not complete current")
	}
	if err := e.Next(background); err != nil {
		t.Fatal(err)
	}
	saved, _ = db.Playback(background, c)
	if saved.Completed || state(t, e).Current.ID != b {
		t.Fatal("manual Next marked completion")
	}
	entry = ready(t, e, p, 40)
	p.event("end-file", "eof", entry)
	eventually(t, func() bool { return state(t, e).Status == Stopped })
	if len(state(t, e).Queue) != 0 {
		t.Fatal("queue did not drain")
	}
	if err := e.Next(background); err != nil {
		t.Fatal(err)
	}
}

func TestStaleAndUnidentifiedEventsCannotChangeNewEpisode(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	old := ready(t, e, p, 10)
	if err := e.PlayEpisode(background, items[1].ID); err != nil {
		t.Fatal(err)
	}
	p.event("end-file", "eof", old)
	p.events <- player.Event{Name: "end-file", Reason: "eof"}
	p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: json.RawMessage("999")}
	current := ready(t, e, p, 2)
	p.event("end-file", "error", old)
	p.event("end-file", "redirect", current)
	p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: json.RawMessage("null")}
	p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: json.RawMessage(`"bad"`)}
	p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: json.RawMessage("-1")}
	// A matching stop is a barrier after the preceding events, not completion.
	p.event("end-file", "stop", current)
	eventually(t, func() bool { return state(t, e).Status == Stopped })
	snapshot := state(t, e)
	if snapshot.Current.ID != items[1].ID || snapshot.Current.Completed || snapshot.Current.PositionSeconds != 2 {
		t.Fatalf("stale event changed playback: %+v", snapshot.Current)
	}
	p.event("end-file", "eof", current)
	saved, _ := db.Playback(background, items[1].ID)
	if saved.Completed {
		t.Fatal("stopped episode marked completed")
	}
}

func TestCompletionAndFailedStreamDoNotAutoplay(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	if err := e.Play(background); !errors.Is(err, ErrNoEpisode) {
		t.Fatalf("empty play: %v", err)
	}
	if err := e.Pause(background); err != nil {
		t.Fatal(err)
	}
	if err := e.Enqueue(background, items[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	entry := ready(t, e, p, 19)
	if err := e.MarkCompleted(background, items[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if state(t, e).Current.ID != items[0].ID || state(t, e).Status != Playing {
		t.Fatal("manual mark changed transport")
	}
	if err := e.MarkCompleted(background, items[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if err := e.MarkCompleted(background, items[1].ID, true); err != nil {
		t.Fatal(err)
	}
	p.event("end-file", "error", entry)
	eventually(t, func() bool { return state(t, e).Status == Failed })
	snapshot := state(t, e)
	if snapshot.Error == "" || snapshot.Current.Completed || len(snapshot.Queue) != 1 {
		t.Fatalf("error skipped queue: %+v", snapshot)
	}
	saved, _ := db.Playback(background, items[0].ID)
	if saved.PositionSeconds != 19 {
		t.Fatal("failure lost known progress")
	}
	if err := e.PlayEpisode(background, items[1].ID); err != nil {
		t.Fatal(err)
	}
	if p.starts[len(p.starts)-1] != 0 || state(t, e).Current.Completed || state(t, e).Error != "" {
		t.Fatal("completed episode did not restart cleanly")
	}
}

func TestQueueValidationAndCanceledRequests(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	if err := e.SetQueue(background, []int64{items[0].ID}); err != nil {
		t.Fatal(err)
	}
	before := state(t, e).Queue
	if err := e.SetQueue(background, []int64{999}); err == nil {
		t.Fatal("unknown queued episode accepted")
	}
	if !reflect.DeepEqual(before, state(t, e).Queue) {
		t.Fatal("bad queue changed state")
	}
	if err := e.PlayEpisode(background, 999); err == nil {
		t.Fatal("unknown episode played")
	}
	if err := e.MarkCompleted(background, 999, true); err == nil {
		t.Fatal("unknown episode completed")
	}
	cancelled, cancel := context.WithCancel(background)
	cancel()
	if err := e.PlayEpisode(cancelled, items[0].ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled action: %v", err)
	}
	if err := e.SetQueue(background, nil); err != nil {
		t.Fatal(err)
	}
	if len(state(t, e).Queue) != 0 {
		t.Fatal("clear queue failed")
	}
	if err := e.Stop(background); err != nil {
		t.Fatal(err)
	}
}

func TestPlayerFailuresStopWithoutCompletion(t *testing.T) {
	for _, op := range []string{"load", "entry", "play", "pause", "position", "stop"} {
		t.Run(op, func(t *testing.T) {
			db, p, items := setup(t)
			e, _ := engineFor(t, db, p)
			if op == "load" || op == "entry" {
				p.setFailure(op)
			}
			err := e.PlayEpisode(background, items[0].ID)
			if op != "load" && op != "entry" {
				if err != nil {
					t.Fatal(err)
				}
				ready(t, e, p, 10)
				p.setFailure(op)
				switch op {
				case "play":
					err = e.Play(background)
				case "pause", "position":
					err = e.Pause(background)
				case "stop":
					err = e.Stop(background)
				}
			}
			if err == nil || state(t, e).Status != Failed || state(t, e).Error == "" {
				t.Fatalf("%s failure ignored: %v", op, err)
			}
			saved, _ := db.Playback(background, items[0].ID)
			if saved.Completed {
				t.Fatal("failure completed episode")
			}
		})
	}
}

func TestInvalidPositionAndDisconnectedPlayer(t *testing.T) {
	db, p, items := setup(t)
	e, _ := engineFor(t, db, p)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	ready(t, e, p, 10)
	p.setPosition(math.NaN())
	if err := e.Pause(background); err == nil {
		t.Fatal("invalid position accepted")
	}
	if p.closed != 1 {
		t.Fatal("failure did not close the player")
	}
	p.setPosition(10)
	if err := e.PlayEpisode(background, items[0].ID); err != nil {
		t.Fatal(err)
	}
	close(p.events)
	eventually(t, func() bool {
		return state(t, e).Status == Failed && state(t, e).Error == player.ErrDisconnected.Error()
	})
}
