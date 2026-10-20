package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/control"
	"podmutt/internal/playback"
	"podmutt/internal/profile"
	"podmutt/internal/theme"
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
	return shell{Model: ui.New(ui.Data{}).WithPlayerDecoration(s.config.PlayerDecoration).WithIcons(s.config.Icons).WithTheme(s.theme).WithSettings(s.settingsData()).WithVersion(Version).WithAnimation(s.ctx, nil).WithProgress(s.ctx, nil, nil).WithActions(func(command string) tea.Cmd {
		return func() tea.Msg {
			var err error
			if strings.HasPrefix(command, "theme:") {
				chosen := strings.TrimPrefix(command, "theme:")
				_ = profile.SetTheme(s.paths.Config, chosen)
				s.config.Theme = chosen
				pal, themePath := theme.Load(s.paths.ConfigDir(), chosen)
				s.theme = pal
				return themeReloadMsg{palette: pal, themeName: chosen, themePath: themePath}
			}
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

// Current audio is projected into visible Queue, but never actor pending storage.
func currentInQueue(state playback.State) bool {
	return state.Current != nil && (!state.Current.Completed || state.Status != playback.Stopped)
}

// Project an authoritative snapshot without arming/discarding subscription commands.
func (m shell) withPlayback(state playback.State) shell {
	if state.Revision != 0 && state.Revision < m.state.Revision {
		return m
	}
	m.state = state
	// Marking history complete does not itself stop current audio. Keep its
	// read-only identity visible until transport actually stops/finishes.
	currentVisible := currentInQueue(state)
	var queue []ui.Item
	for _, episode := range state.Queue {
		if currentVisible && episode.ID == state.Current.ID {
			continue
		}
		queue = append(queue, queueItem(episode))
	}
	player := ui.Player{Status: string(state.Status), Position: "0:00", Duration: "--:--", Error: state.Error, CanNext: len(state.Queue) > 0, CanSeek: state.CanSeek}
	if episode := state.Current; episode != nil {
		if currentVisible {
			item := queueItem(*episode)
			item.Current = true
			queue = append(queue, item)
		}
		player.HasCurrent, player.Title, player.Channel = true, cleanTitle(episode.Title), cleanShow(episode.FeedTitle)
		player.PositionSeconds, player.DurationSeconds, player.ClockAt = episode.PositionSeconds, float64(episode.DurationSeconds), state.ClockAt
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
	return m
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
		m = m.withPlayback(msg.state)
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
	var animation, progress tea.Cmd
	m.Model, animation = m.Model.SyncAnimation()
	m.Model, progress = m.Model.SyncProgress()
	return m, tea.Batch(cmd, animation, progress)
}
