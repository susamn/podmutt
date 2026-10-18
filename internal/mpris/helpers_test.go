package mpris

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"podmutt/internal/playback"
	"podmutt/internal/store"
)

var background = context.Background()
var failure = errors.New("injected failure")

type fakeBackend struct {
	mu      sync.Mutex
	state   playback.State
	calls   []string
	fail    string
	streams map[chan playback.State]bool
}

func fixture() *fakeBackend {
	return &fakeBackend{state: stateFixture(), streams: make(map[chan playback.State]bool)}
}
func clone(state playback.State) playback.State {
	if state.Current != nil {
		copy := *state.Current
		state.Current = &copy
	}
	state.Queue = append([]store.Episode(nil), state.Queue...)
	return state
}
func (f *fakeBackend) Snapshot(context.Context) (playback.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail == "snapshot" {
		return playback.State{}, failure
	}
	return clone(f.state), nil
}
func (f *fakeBackend) Watch(ctx context.Context) (<-chan playback.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail == "watch" {
		return nil, failure
	}
	stream := make(chan playback.State, 1)
	stream <- clone(f.state)
	f.streams[stream] = true
	context.AfterFunc(ctx, func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.streams[stream] {
			delete(f.streams, stream)
			close(stream)
		}
	})
	return stream, nil
}
func (f *fakeBackend) publish() {
	for stream := range f.streams {
		select {
		case <-stream:
		default:
		}
		stream <- clone(f.state)
	}
}
func (f *fakeBackend) set(state playback.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = clone(state)
	f.publish()
}
func (f *fakeBackend) act(command string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail == command {
		return failure
	}
	f.calls = append(f.calls, command)
	switch command {
	case "play":
		f.state.Status = playback.Playing
	case "pause":
		f.state.Status = playback.Paused
	case "stop":
		f.state.Status = playback.Stopped
	case "next":
		current := f.state.Queue[0]
		f.state.Current = &current
		f.state.Queue = f.state.Queue[1:]
		f.state.Status = playback.Playing
	}
	f.publish()
	return nil
}
func (f *fakeBackend) Play(context.Context) error  { return f.act("play") }
func (f *fakeBackend) Pause(context.Context) error { return f.act("pause") }
func (f *fakeBackend) Stop(context.Context) error  { return f.act("stop") }
func (f *fakeBackend) Next(context.Context) error  { return f.act("next") }
func serviceFor(f *fakeBackend) *Service {
	return &Service{ctx: background, backend: f, values: playerValues(clone(f.state)), updated: time.Now()}
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
func property(t *testing.T, object dbus.BusObject, name string) any {
	t.Helper()
	value, err := object.GetProperty(playerInterface + "." + name)
	if err != nil {
		t.Fatal(err)
	}
	return value.Value()
}
