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
	"podmutt/internal/discovery"
	"podmutt/internal/feedback"
	"podmutt/internal/playback"
	"podmutt/internal/profile"
	"podmutt/internal/theme"
	"podmutt/internal/ui"
)

func userPaths() (profile.Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return profile.Paths{}, &feedback.Fault{Code: feedback.Input, Safe: "Set HOME to your absolute home directory before starting podmutt.", Cause: err}
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
type actionSucceeded struct{}
type settingsError struct {
	setting string
	err     error
}
type shell struct {
	ui.Model
	s     *session
	state playback.State
}

type countryChangedMsg struct {
	country string
}
type themeSignal struct{}
type themeReloadMsg struct {
	reload    bool
	palette   theme.Palette
	themeName string
	themePath string
}

func (m libraryShell) updateSettings(msg tea.Msg) (libraryShell, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case countryChangedMsg:
		m.s.config.Country = msg.country
		if apple, ok := m.browse.directory.(*discovery.Apple); ok {
			apple.SetCountry(msg.country)
		}
		m.Model = m.Model.WithSettings(m.s.settingsData())
		m.Model = m.Model.ClearFeedback("Save country")
		m.notice = "Country saved: " + strings.ToUpper(msg.country)
		if m.browse.active {
			m.browse.country = msg.country
			m.browse.loaded = false
			return m, m.searchDirectory(m.browse.query), true
		}
		return m, nil, true
	case settingsError:
		m.Model = m.Model.WithSettings(m.s.settingsData())
		message := feedback.For("Save "+msg.setting, msg.err)
		message.Summary = "Could not save " + msg.setting + " to config.toml."
		message.Action = "Check free disk space and that config.toml is a writable file, not a directory. Previous setting remains active. Config: " + feedback.PublicLabel(m.s.paths.Config)
		m.notice = message.Text()
		m.Model = m.Model.WithFeedback(message)
		return m, nil, true
	case themeSignal:
		name, paths, ctx := m.s.config.Theme, m.s.paths, m.s.ctx
		return m, tea.Batch(m.waitSignal(), work(ctx, "Reloading theme…", func(context.Context) tea.Msg {
			pal, path := theme.Load(paths.ConfigDir(), name)
			return themeReloadMsg{palette: pal, themeName: name, themePath: path, reload: true}
		})), true
	case themeReloadMsg:
		if msg.reload && msg.themeName != m.s.config.Theme {
			return m, nil, true
		}
		if !msg.reload {
			m.Model = m.Model.ClearFeedback("Save theme")
		}
		m.s.theme = msg.palette
		if msg.themeName != "" {
			m.s.config.Theme = msg.themeName
		}
		m.Model = m.Model.WithTheme(msg.palette)
		if !msg.reload {
			m.Model = m.Model.WithSettings(m.s.settingsData())
		}
		return m, nil, true
	default:
		return m, nil, false
	}
}

func newShell(s *session) shell {
	return shell{Model: ui.New(ui.Data{}).WithPlayerDecoration(s.config.PlayerDecoration).WithIcons(s.config.Icons).WithTheme(s.theme).WithSettings(s.settingsData()).WithVersion(Version).WithAnimation(s.ctx, nil).WithProgress(s.ctx, nil, nil).WithActivityClock(s.ctx, nil).WithActions(func(act ui.Action) tea.Cmd {
		cfg := s.config // capture on the UI thread; workers never mutate session settings
		return work(s.ctx, commandLabel(string(act.Kind)), func(ctx context.Context) tea.Msg {
			var err error
			switch act.Kind {
			case ui.ActionSetTheme:
				chosen := act.Payload
				if err := profile.SetTheme(s.paths.Config, chosen); err != nil {
					return settingsError{"theme", err}
				}
				pal, themePath := theme.Load(s.paths.ConfigDir(), chosen)
				return themeReloadMsg{palette: pal, themeName: chosen, themePath: themePath}
			case ui.ActionCycleCountry:
				countries := []string{"us", "de", "gb", "fr", "ca", "au", "jp", "es", "it", "nl"}
				curr := strings.ToLower(cfg.Country)
				nextIndex := 0
				for idx, c := range countries {
					if c == curr {
						nextIndex = (idx + 1) % len(countries)
						break
					}
				}
				chosen := countries[nextIndex]
				if err := profile.SetCountry(s.paths.Config, chosen); err != nil {
					return settingsError{"country", err}
				}
				return countryChangedMsg{country: chosen}
			case ui.ActionSeekForward, ui.ActionSeekBackward:
				delta := 15.0
				if act.Kind == ui.ActionSeekBackward {
					delta = -15
				}
				err = s.playback.SeekBy(ctx, delta)
			case ui.ActionToggle:
				err = s.playback.Toggle(ctx, 0)
			default:
				_, err = control.Execute(ctx, s.playback, string(act.Kind))
			}
			if err != nil {
				return actionError{err}
			}
			return actionSucceeded{}
		})
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
	if state.Error != "" && (state.Error != m.state.Error || state.ErrorRevision != m.state.ErrorRevision) {
		m.Model = m.Model.WithFeedback(feedback.Message{Scope: "Playback", Code: feedback.PlayerFailure, Severity: "error", Summary: state.Error})
	} else if state.Error == "" && m.state.Error != "" {
		m.Model = m.Model.ClearFeedback("Playback")
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
	player := ui.Player{Status: string(state.Status), Position: "0:00", Duration: "--:--", Error: state.Error, Volume: state.Volume, HasVolume: true, CanNext: len(state.Queue) > 0, CanSeek: state.CanSeek}
	if episode := state.Current; episode != nil {
		if currentVisible {
			item := queueItem(*episode)
			item.Current = true
			queue = append(queue, item)
		}
		player.HasCurrent, player.Title, player.Channel = true, cleanTitle(episode.Title), cleanShow(episode.FeedTitle)
		player.PositionSeconds, player.DurationSeconds, player.ClockAt = episode.PositionSeconds, float64(episode.DurationSeconds), state.ClockAt
		player.Position = clock(episode.PositionSeconds)
		player.Chapters = episode.ParsedChapters()
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
		player.Warning = "Desktop media keys are unavailable. Use the in-app playback controls."
	}
	m.Model = m.Model.WithPlayback(queue, player)
	return m
}

func (m shell) wait() tea.Cmd {
	return func() tea.Msg { state, ok := <-m.s.playback.Updates(); return playbackUpdate{state, ok} }
}
func (m shell) Init() tea.Cmd { return tea.Batch(m.wait(), m.waitActivity()) }
func (m shell) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case activityChanged:
		m.Model = m.Model.WithActivity(msg.snapshot)
		var tick tea.Cmd
		m.Model, tick = m.Model.SyncActivity()
		return m, tea.Batch(m.waitActivity(), tick)
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
	case actionSucceeded:
		m.Model = m.Model.ClearFeedback("Playback").WithPlayerError("")
		return m, nil
	case actionError:
		message := feedback.For("Playback", msg.error)
		m.Model = m.Model.WithFeedback(message).WithPlayerError(message.Text())
		return m, nil
	}
	updated, cmd := m.Model.Update(msg)
	m.Model = updated.(ui.Model)
	var animation, progress tea.Cmd
	m.Model, animation = m.Model.SyncAnimation()
	m.Model, progress = m.Model.SyncProgress()
	return m, tea.Batch(cmd, animation, progress)
}
