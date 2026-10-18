package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"

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
	return shell{Model: ui.New(ui.Data{}).WithIcons(s.config.Icons).WithAnimation(s.ctx, nil).WithProgress(s.ctx, nil, nil).WithActions(func(command string) tea.Cmd {
		return func() tea.Msg {
			var err error
			if command == "seek-forward" || command == "seek-backward" {
				delta := 15.0
				if command == "seek-backward" {
					delta = -15
				}
				err = s.playback.SeekBy(s.ctx, delta)
			} else if command == "toggle" {
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
		m.state = msg.state
		var queue []ui.Item
		if current := msg.state.Current; current != nil && !current.Completed {
			queue = append(queue, stackItem(*current, string(msg.state.Status)))
		}
		for _, episode := range msg.state.Queue {
			queue = append(queue, stackItem(episode, "queued"))
		}
		player := ui.Player{Status: string(msg.state.Status), Position: "0:00", Duration: "--:--", Error: msg.state.Error, CanNext: len(msg.state.Queue) > 0, CanSeek: msg.state.CanSeek}
		if episode := msg.state.Current; episode != nil {
			player.HasCurrent = true
			player.PositionSeconds = episode.PositionSeconds
			player.DurationSeconds = float64(episode.DurationSeconds)
			player.ClockAt = msg.state.ClockAt
			player.Title = library.PlainText(episode.Title)
			player.Position = clock(episode.PositionSeconds)
			if episode.DurationSeconds > 0 {
				player.Duration = clock(float64(episode.DurationSeconds))
				player.Progress = episode.PositionSeconds / float64(episode.DurationSeconds)
			}
		}
		mediaError := m.s.mediaError
		if m.s.media != nil {
			mediaError = errors.Join(mediaError, m.s.media.Err())
		}
		mediaError = errors.Join(mediaError, m.s.nativeMedia.Err())
		if mediaError != nil {
			player.Warning = mediaError.Error()
		}
		m.Model = m.Model.WithPlayback(queue, player)
		var animation tea.Cmd
		m.Model, animation = m.Model.SyncAnimation()
		var progress tea.Cmd
		m.Model, progress = m.Model.SyncProgress()
		return m, tea.Batch(m.wait(), animation, progress)
	case actionError:
		m.Model = m.Model.WithPlayerError(msg.Error())
		return m, nil
	}
	updated, cmd := m.Model.Update(msg)
	m.Model = updated.(ui.Model)
	var progress tea.Cmd
	m.Model, progress = m.Model.SyncProgress()
	return m, tea.Batch(cmd, progress)
}
