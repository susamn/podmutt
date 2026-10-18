package macmedia

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"podmutt/internal/playback"
	"podmutt/internal/store"
)

type fakeBackend struct {
	updates     chan playback.State
	calls       chan string
	err         error
	watchErr    error
	watchCancel context.CancelFunc
	block       bool
}

func (b *fakeBackend) Watch(context.Context) (<-chan playback.State, error) {
	if b.watchCancel != nil {
		b.watchCancel()
	}
	return b.updates, b.watchErr
}
func (b *fakeBackend) Snapshot(context.Context) (playback.State, error) { return playback.State{}, nil }
func (b *fakeBackend) act(ctx context.Context, action string) error {
	b.calls <- action
	if b.block {
		<-ctx.Done()
		return ctx.Err()
	}
	return b.err
}
func (b *fakeBackend) Play(ctx context.Context) error  { return b.act(ctx, "play") }
func (b *fakeBackend) Pause(ctx context.Context) error { return b.act(ctx, "pause") }
func (b *fakeBackend) Stop(ctx context.Context) error  { return b.act(ctx, "stop") }
func (b *fakeBackend) Next(ctx context.Context) error  { return b.act(ctx, "next") }
func (b *fakeBackend) Toggle(ctx context.Context, id int64) error {
	if id != 0 {
		panic("selected rather than current episode")
	}
	return b.act(ctx, "toggle")
}

type fakeDriver struct {
	callback  func(string) bool
	updates   chan metadata
	closed    atomic.Int32
	err       error
	updateErr error
}

func (d *fakeDriver) Install(callback func(string) bool) error { d.callback = callback; return d.err }
func (d *fakeDriver) Update(m metadata) error                  { d.updates <- m; return d.updateErr }
func (d *fakeDriver) Close()                                   { d.closed.Add(1) }
func fixtures() (*fakeBackend, *fakeDriver) {
	return &fakeBackend{updates: make(chan playback.State, 16), calls: make(chan string, 32)}, &fakeDriver{updates: make(chan metadata, 32)}
}
func TestSharedRemoteMappingMetadataAndCleanup(t *testing.T) {
	b, d := fixtures()
	s, err := start(context.Background(), b, d)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	b.updates <- playback.State{Status: playback.Playing, Current: &store.Episode{Title: "Current", PositionSeconds: 42, DurationSeconds: 90}, Queue: []store.Episode{{ID: 2}}}
	select {
	case m := <-d.updates:
		if !m.Playing || !m.HasCurrent || !m.CanNext || m.Title != "Current" || m.Position != 42 || m.Duration != 90 {
			t.Fatal(m)
		}
	case <-time.After(time.Second):
		t.Fatal("metadata stalled")
	}
	for _, action := range []string{"play", "pause", "toggle", "stop", "next"} {
		if !d.callback(action) {
			t.Fatal("not accepted", action)
		}
		select {
		case got := <-b.calls:
			if got != action {
				t.Fatal(got)
			}
		case <-time.After(time.Second):
			t.Fatal("action stalled")
		}
	}
	if d.callback("seek") {
		t.Fatal("unsupported action")
	}
	s.Close()
	s.Close()
	if d.callback("play") || d.closed.Load() != 1 {
		t.Fatal("handler retained after close")
	}
	if s.Err() != nil {
		t.Fatal(s.Err())
	}
	m := info(playback.State{Status: playback.Paused, Current: &store.Episode{ID: 1}, Queue: []store.Episode{{ID: 2}}})
	if !m.Paused || m.Playing || m.CanNext {
		t.Fatal("paused capability", m)
	}
	if info(playback.State{}).HasCurrent {
		t.Fatal("empty metadata")
	}
}
func TestWatchInstallActionErrorsAndBoundedQueue(t *testing.T) {
	sentinel := errors.New("failed")
	b, d := fixtures()
	b.watchErr = sentinel
	if _, err := start(context.Background(), b, d); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	b, d = fixtures()
	d.err = sentinel
	if _, err := start(context.Background(), b, d); !errors.Is(err, sentinel) || d.closed.Load() != 1 {
		t.Fatal("partial install leaked", err)
	}
	b, d = fixtures()
	b.err = sentinel
	s, err := start(context.Background(), b, d)
	if err != nil {
		t.Fatal(err)
	}
	d.callback("play")
	<-b.calls
	deadline := time.Now().Add(time.Second)
	for s.Err() == nil && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !errors.Is(s.Err(), sentinel) {
		t.Fatal("action error lost")
	}
	s.Close()
	b, d = fixtures()
	b.block = true
	s, err = start(context.Background(), b, d)
	if err != nil {
		t.Fatal(err)
	}
	d.callback("play")
	<-b.calls
	accepted := 0
	for i := 0; i < 20; i++ {
		if d.callback("next") {
			accepted++
		}
	}
	if accepted != 8 {
		t.Fatal("unbounded/non-deterministic queue", accepted)
	}
	s.Close()
	if d.callback("play") {
		t.Fatal("closed callback accepted")
	}
	b, d = fixtures()
	s, err = start(context.Background(), b, d)
	if err != nil {
		t.Fatal(err)
	}
	close(b.updates)
	select {
	case <-s.done:
	case <-time.After(time.Second):
		t.Fatal("closed watcher leaked")
	}
	if d.callback("play") {
		t.Fatal("closed watcher still active")
	}
	s.Close()
	var absent *Service
	absent.Close()
	if absent.Err() != nil {
		t.Fatal("nil error")
	}
}
