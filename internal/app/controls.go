package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/control"
	"podmutt/internal/playback"
	"podmutt/internal/profile"
	"podmutt/internal/ui"
)

func userPaths() (profile.Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return profile.Paths{}, err
	}
	return profile.Resolve(runtime.GOOS, home, os.Getenv)
}

func Command(command string, output io.Writer) error {
	paths, err := userPaths()
	if err != nil {
		return err
	}
	status, err := control.Call(context.Background(), paths, command)
	if err != nil {
		return err
	}
	if command == "status" {
		return json.NewEncoder(output).Encode(status)
	}
	return nil
}

type playbackUpdate struct {
	state playback.State
	ok    bool
}
type actionError struct{ error }
type shell struct {
	ui.Model
	s *session
}

func newShell(s *session) shell {
	return shell{ui.New(ui.Data{}).WithActions(func(command string) tea.Cmd {
		return func() tea.Msg {
			if command == "toggle" {
				state, err := s.playback.Snapshot(s.ctx)
				if err != nil {
					return actionError{err}
				}
				command = "play"
				if state.Status == playback.Playing {
					command = "pause"
				}
			}
			_, err := control.Execute(s.ctx, s.playback, command)
			if err != nil {
				return actionError{err}
			}
			return nil
		}
	}), s}
}
func (m shell) wait() tea.Cmd {
	return func() tea.Msg { state, ok := <-m.s.playback.Updates(); return playbackUpdate{state, ok} }
}
func (m shell) Init() tea.Cmd { return m.wait() }
func (m shell) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case playbackUpdate:
		if !msg.ok {
			return m, nil
		}
		title := ""
		if msg.state.Current != nil {
			title = " · " + msg.state.Current.Title
		}
		text := fmt.Sprintf("%s%s", msg.state.Status, title)
		if msg.state.Error != "" {
			text += " · " + msg.state.Error
		}
		mediaError := m.s.mediaError
		if m.s.media != nil {
			mediaError = m.s.media.Err()
		}
		if mediaError != nil {
			text += " · " + mediaError.Error()
		}
		m.Model = m.Model.WithPlayer(text)
		return m, m.wait()
	case actionError:
		m.Model = m.Model.WithPlayer(msg.Error())
		return m, nil
	}
	updated, cmd := m.Model.Update(msg)
	m.Model = updated.(ui.Model)
	return m, cmd
}
