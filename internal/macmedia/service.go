// Package macmedia maps public macOS remote commands to the foreground actor.
package macmedia

import (
	"context"
	"errors"
	"sync"
	"time"

	"podmutt/internal/control"
	"podmutt/internal/playback"
)

var ErrUnavailable = errors.New("macOS media controls unavailable: use a bundled native CGO build")

type backend interface {
	control.Actions
	Toggle(context.Context, int64) error
	Watch(context.Context) (<-chan playback.State, error)
	SeekBy(context.Context, float64) error
}
type metadata struct {
	Title                                string
	Position, Duration                   float64
	Playing, Paused, HasCurrent, CanNext bool
	CanSeek                              bool
}
type driver interface {
	Install(func(string) bool) error
	Update(metadata) error
	Close()
}
type Service struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
	driver driver
	mu     sync.Mutex
	err    error
}

func info(s playback.State) metadata {
	m := metadata{Playing: s.Status == playback.Playing, Paused: s.Status == playback.Paused, CanNext: len(s.Queue) > 0, CanSeek: s.CanSeek}
	if s.Current != nil {
		title := []rune(s.Current.Title)
		m.Title = string(title[:min(256, len(title))])
		m.Position = s.Current.PositionSeconds
		m.Duration = float64(s.Current.DurationSeconds)
		m.HasCurrent = true
	}
	return m
}
func start(ctx context.Context, b backend, d driver) (*Service, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	updates, err := b.Watch(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		cancel()
		return nil, err
	}
	actions := make(chan string, 8)
	accepted := func(action string) bool {
		switch action {
		case "play", "pause", "stop", "next", "toggle", "seek-forward", "seek-backward":
		default:
			return false
		}
		if ctx.Err() != nil {
			return false
		}
		select {
		case actions <- action:
			return true
		default:
			return false
		}
	}
	if err := d.Install(accepted); err != nil {
		cancel()
		d.Close()
		return nil, err
	}
	s := &Service{cancel: cancel, done: make(chan struct{}), driver: d}
	go func() {
		defer close(s.done)
		defer d.Close()
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case state, ok := <-updates:
				if !ok {
					return
				}
				if err := d.Update(info(state)); err != nil {
					s.mu.Lock()
					s.err = err
					s.mu.Unlock()
					return
				}
			case action := <-actions:
				actionCtx, stop := context.WithTimeout(ctx, 3*time.Second)
				var err error
				if action == "seek-forward" || action == "seek-backward" {
					delta := 15.0
					if action == "seek-backward" {
						delta = -15
					}
					err = b.SeekBy(actionCtx, delta)
				} else if action == "toggle" {
					err = b.Toggle(actionCtx, 0)
				} else {
					_, err = control.Execute(actionCtx, b, action)
				}
				stop()
				if err != nil {
					s.mu.Lock()
					s.err = err
					s.mu.Unlock()
				}
			}
		}
	}()
	return s, nil
}
func (s *Service) Err() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}
func (s *Service) Close() {
	if s != nil {
		s.once.Do(func() { s.cancel(); <-s.done })
	}
}
