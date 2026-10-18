package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/control"
	"podmutt/internal/library"
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
	s     *session
	state playback.State
}

func newShell(s *session) shell {
	return shell{Model: ui.New(ui.Data{}).WithActions(func(command string) tea.Cmd {
		return func() tea.Msg {
			var err error
			if command == "toggle" {
				err = s.playback.Toggle(s.ctx, 0)
			} else {
				_, err = control.Execute(s.ctx, s.playback, command)
			}
			if err != nil {
				return actionError{err}
			}
			return nil
		}
	}), s: s}
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
		m.state = msg.state
		var queue []ui.Item
		for _, episode := range msg.state.Queue {
			queue = append(queue, ui.Item{ID: strconv.FormatInt(episode.ID, 10), Title: library.PlainText(episode.Title)})
		}
		duration := "--:--"
		position := float64(0)
		if episode := msg.state.Current; episode != nil {
			position = episode.PositionSeconds
			if episode.DurationSeconds > 0 {
				duration = clock(float64(episode.DurationSeconds))
			}
		}
		text := fmt.Sprintf("%s%s · %s / %s", msg.state.Status, title, clock(position), duration)
		if msg.state.Error != "" {
			text += " · " + msg.state.Error
		}
		m.Model = m.Model.WithPlayback(queue, text)
		return m, m.wait()
	case actionError:
		m.Model = m.Model.WithPlayer(msg.Error())
		return m, nil
	}
	updated, cmd := m.Model.Update(msg)
	m.Model = updated.(ui.Model)
	return m, cmd
}
