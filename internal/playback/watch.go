package playback

import (
	"context"
	"sync"
)

type subscriptions struct {
	sync.Mutex
	closed  bool
	streams map[chan State]func() bool
}

// Watch adds an independent, coalesced snapshot stream. Unlike Updates, watchers
// never compete with the TUI. Cancellation or engine shutdown closes the stream.
func (e *Engine) Watch(ctx context.Context) (<-chan State, error) {
	stream := make(chan State, 1)
	e.watchers.Lock()
	if e.watchers.closed {
		e.watchers.Unlock()
		return nil, ErrClosed
	}
	if e.watchers.streams == nil {
		e.watchers.streams = make(map[chan State]func() bool)
	}
	e.watchers.streams[stream] = context.AfterFunc(ctx, func() { e.removeWatcher(stream) })
	e.watchers.Unlock()
	// Publish in actor order; never overwrite a newer event with a stale initial
	// snapshot acquired before registration.
	if _, err := e.submit(ctx, func(context.Context) error { return nil }); err != nil {
		e.removeWatcher(stream)
		return nil, err
	}
	return stream, nil
}
func (e *Engine) removeWatcher(stream chan State) {
	e.watchers.Lock()
	defer e.watchers.Unlock()
	if stop, ok := e.watchers.streams[stream]; ok {
		stop()
		delete(e.watchers.streams, stream)
		close(stream)
	}
}
func (e *Engine) closeWatchers() {
	e.watchers.Lock()
	defer e.watchers.Unlock()
	e.watchers.closed = true
	for stream, stop := range e.watchers.streams {
		stop()
		close(stream)
	}
	e.watchers.streams = nil
}
func (e *Engine) publishWatchers() {
	e.watchers.Lock()
	defer e.watchers.Unlock()
	for stream := range e.watchers.streams {
		offer(stream, e.snapshot())
	}
}
func offer(stream chan State, state State) {
	select {
	case stream <- state:
	default:
		select {
		case <-stream:
		default:
		}
		select {
		case stream <- state:
		default:
		}
	}
}
