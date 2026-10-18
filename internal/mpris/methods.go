package mpris

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"
	"podmutt/internal/control"
	"podmutt/internal/playback"
)

const actionTimeout = 3 * time.Second

type Backend interface {
	control.Actions
	Watch(context.Context) (<-chan playback.State, error)
}

type rootMethods struct{}

func (*rootMethods) Raise() *dbus.Error { return busError("NotSupported") }
func (*rootMethods) Quit() *dbus.Error  { return busError("NotSupported") }

type playerMethods struct{ s *Service }

func (p *playerMethods) Play() *dbus.Error      { return p.s.act("play") }
func (p *playerMethods) Pause() *dbus.Error     { return p.s.act("pause") }
func (p *playerMethods) PlayPause() *dbus.Error { return p.s.act("toggle") }
func (p *playerMethods) Stop() *dbus.Error      { return p.s.act("stop") }
func (p *playerMethods) Next() *dbus.Error      { return p.s.act("next") }
func (*playerMethods) Previous() *dbus.Error    { return nil }
func seek(int64) *dbus.Error                    { return nil }

// Export Seek as a closure: a Go method named Seek conflicts with go vet's
// io.Seeker signature check, while MPRIS requires a different wire signature.
func (p *playerMethods) table() map[string]any {
	return map[string]any{"Play": p.Play, "Pause": p.Pause, "PlayPause": p.PlayPause,
		"Stop": p.Stop, "Next": p.Next, "Previous": p.Previous, "Seek": seek,
		"SetPosition": p.SetPosition, "OpenUri": p.OpenUri}
}
func (*playerMethods) SetPosition(dbus.ObjectPath, int64) *dbus.Error { return nil }
func (*playerMethods) OpenUri(string) *dbus.Error                     { return busError("NotSupported") }

func (s *Service) act(command string) *dbus.Error {
	ctx, cancel := context.WithTimeout(s.ctx, actionTimeout)
	defer cancel()
	state, err := s.backend.Snapshot(ctx)
	if err != nil {
		return busError("Failed")
	}
	switch command {
	case "toggle":
		if state.Current == nil {
			return busError("NotSupported")
		}
		command = "play"
		if state.Status == playback.Playing {
			command = "pause"
		}
	case "play":
		if state.Current == nil || state.Status == playback.Playing {
			return nil
		}
	case "pause":
		if state.Status != playback.Playing {
			return nil
		}
	case "next":
		// The core Next starts a queued episode. Do not silently resume a paused
		// podcast: advertise/allow this capability only while already playing.
		if state.Status != playback.Playing || len(state.Queue) == 0 {
			return nil
		}
	}
	if _, err := control.Execute(ctx, s.backend, command); err != nil {
		return busError("Failed")
	}
	return nil
}
