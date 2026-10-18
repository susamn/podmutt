package mpris

import (
	"context"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"podmutt/internal/playback"
)

type Service struct {
	ctx     context.Context
	cancel  context.CancelFunc
	backend Backend
	conn    *dbus.Conn
	done    chan struct{}
	mu      sync.Mutex
	values  values
	updated time.Time
	err     error
	emit    func(string, ...any) error
}

// Listen exports on a private connection to an explicit Unix session-bus
// address. Close releases the name/handlers without affecting playback.
func Listen(parent context.Context, address string, backend Backend) (service *Service, err error) {
	lifetime, cancel := context.WithCancel(parent)
	startup, stopStartup := context.WithTimeout(lifetime, startupTimeout)
	defer stopStartup()
	stopTimeout := context.AfterFunc(startup, cancel)
	defer stopTimeout()
	conn, err := connect(startup, lifetime, address)
	if err != nil {
		cancel()
		return nil, ErrUnavailable
	}
	success := false
	defer func() {
		if !success {
			cancel()
			conn.Close()
		}
	}()
	updates, err := backend.Watch(lifetime)
	if err != nil {
		return nil, ErrUnavailable
	}
	var initial playback.State
	select {
	case state, ok := <-updates:
		if !ok {
			return nil, ErrUnavailable
		}
		initial = state
	case <-startup.Done():
		return nil, ErrUnavailable
	}
	service = &Service{ctx: lifetime, cancel: cancel, backend: backend, conn: conn, done: make(chan struct{}), values: playerValues(initial), updated: time.Now()}
	service.emit = func(signal string, args ...any) error { return conn.Emit(Path, signal, args...) }
	if err := service.export(); err != nil {
		return nil, ErrUnavailable
	}
	reply, err := conn.RequestName(Name, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner || startup.Err() != nil {
		return nil, ErrUnavailable
	}
	success = true
	go service.run(updates)
	return service, nil
}
func (s *Service) run(updates <-chan playback.State) {
	defer close(s.done)
	defer s.cancel()
	defer s.conn.Close()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.conn.Context().Done():
			if s.ctx.Err() == nil {
				s.unavailable()
			}
			return
		case state, ok := <-updates:
			if !ok {
				if s.ctx.Err() == nil {
					s.unavailable()
				}
				return
			}
			if err := s.update(state, time.Now()); err != nil {
				s.unavailable()
				return
			}
		}
	}
}
func (s *Service) unavailable() { s.mu.Lock(); s.err = ErrUnavailable; s.mu.Unlock() }
func (s *Service) Err() error   { s.mu.Lock(); defer s.mu.Unlock(); return s.err }
func (s *Service) Close() error { s.cancel(); <-s.done; return s.Err() }

func (s *Service) update(state playback.State, now time.Time) error {
	after := playerValues(state)
	s.mu.Lock()
	before, elapsed := s.values, now.Sub(s.updated)
	s.values, s.updated = after, now
	s.mu.Unlock()
	updates := changed(before, after)
	if len(updates) > 0 {
		if err := s.emit(propertiesInterface+".PropertiesChanged", playerInterface, updates, []string{}); err != nil {
			return err
		}
	}
	if jumped(before, after, elapsed) {
		return s.emit(playerInterface+".Seeked", after["Position"].Value().(int64))
	}
	return nil
}
