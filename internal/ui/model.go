// Package ui contains the terminal model, independent of feeds, storage, and mpv.
package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/theme"
)

type Panel int

const (
	Podcasts Panel = iota
	Episodes
	Queue
	Details
	PlayerPanel
)

type Item struct {
	ID              string
	Title           string
	PlainTitle      string
	Channel         string
	Date            string
	Duration        string
	Position        string
	Remains         string
	PositionSeconds float64
	DurationSeconds float64
	Queued          bool
	Completed       bool
	State           string
	Index           int
}

type Dialog struct{ Title, Body, Hint string }

type Data struct {
	Dialog                       *Dialog
	Podcasts, Episodes, Queue    []Item
	Details                      string
	Player                       Player
	EpisodeLabel, Notice, Prompt string
	PodcastLabel, ContextHint    string
	PodcastHint                  string
	PodcastEmpty                 string
}

type SettingsData struct {
	ActiveTheme string
	Themes      []string
	Selected    int
	ConfigPath  string
	ThemePath   string
	MPVPath     string
	RefreshMin  int
	Icons       string
}

type Model struct {
	version       string
	theme         theme.Palette
	settings      bool
	settingsData  SettingsData
	colOffset     [3]int
	icons         string
	animation     animation
	progress      progressClock
	data          Data
	width, height int
	focus         Panel
	lastVisible   Panel
	selected      [3]int
	filters       [3]string
	editing       bool
	previous      string
	help          bool
	actions       func(string) tea.Cmd
	detailScroll  int
	playerButton  int
	artColumns    int
	artRows       []string
}

func New(data Data) Model     { return Model{data: data} }
func (m Model) Init() tea.Cmd { return nil }

func (m Model) WithVersion(v string) Model { m.version = v; return m }
func (m Model) Version() string {
	if m.version == "" {
		return "dev"
	}
	return m.version
}
func (m Model) WithTheme(p theme.Palette) Model { m.theme = p; return m }
func (m Model) Theme() theme.Palette {
	if m.theme.Accent == "" {
		return theme.Default()
	}
	return m.theme
}
func (m Model) WithSettings(s SettingsData) Model {
	m.settingsData = s
	for i, t := range s.Themes {
		if t == s.ActiveTheme {
			m.settingsData.Selected = i
			break
		}
	}
	return m
}
func (m Model) SettingsOpen() bool                             { return m.settings }
func (m Model) OpenSettings() Model                            { m.settings = true; return m }
func (m Model) CloseSettings() Model                           { m.settings = false; return m }
func (m Model) WithActions(actions func(string) tea.Cmd) Model { m.actions = actions; return m }
func (m Model) WithPlayerError(text string) Model              { m.data.Player.Error = text; return m }
func (m Model) Focus() Panel                                   { return m.focus }
func (m Model) WithFocus(panel Panel) Model                    { m.setFocus(panel); return m }
func (m Model) Select(panel Panel, id string) Model {
	for i, item := range m.items(panel) {
		if item.ID == id {
			m.selected[panel] = i
			break
		}
	}
	return m
}
func (m Model) Busy() bool {
	return m.editing || m.help || m.settings || m.data.Dialog != nil
}
func (m Model) Visible(panel Panel) []Item      { return m.items(panel) }
func (m Model) WithDialog(dialog *Dialog) Model { m.data.Dialog = dialog; return m }
func (m Model) WithPlayback(queue []Item, player Player) Model {
	data := m.data
	data.Queue, data.Player = queue, player
	updated, _ := m.Update(data)
	return updated.(Model)
}
func (m Model) Selected(panel Panel) (Item, bool) {
	items := m.items(panel)
	if len(items) == 0 {
		return Item{}, false
	}
	return items[m.selected[panel]], true
}
func (m Model) ClearFilter(panel Panel) Model {
	if panel >= Podcasts && panel <= Queue {
		m.filters[panel] = ""
		m.clamp()
	}
	return m
}
func (m Model) Reset(panel Panel) Model {
	if panel >= Podcasts && panel <= Queue {
		m.selected[panel] = 0
		m.colOffset[panel] = 0
	}
	return m
}

// WithLibrary replaces library fields while preserving playback-owned fields.
func (m Model) WithLibrary(data Data) Model {
	data.Player, data.Queue, data.Details = m.data.Player, m.data.Queue, m.data.Details
	data.Dialog = m.data.Dialog
	updated, _ := m.Update(data)
	return updated.(Model)
}
func (m Model) WithStatus(notice, prompt string) Model {
	m.data.Notice, m.data.Prompt = notice, prompt
	return m
}
func (m Model) WithDetails(text string) Model {
	if text != m.data.Details {
		m.detailScroll = 0
	}
	m.data.Details = text
	return m
}

func (m Model) compact() bool { return m.width < 100 || m.height < 24 }

func (m Model) items(panel Panel) []Item {
	var source []Item
	switch panel {
	case Podcasts:
		source = m.data.Podcasts
	case Episodes:
		source = m.data.Episodes
	case Queue:
		source = m.data.Queue
	default:
		return nil
	}
	query := strings.ToLower(m.filters[panel])
	var items []Item
	for _, item := range source {
		if strings.Contains(strings.ToLower(item.Title), query) {
			items = append(items, item)
		}
	}
	return items
}

func (m *Model) clamp() {
	for panel := Podcasts; panel <= Queue; panel++ {
		m.selected[panel] = max(0, min(m.selected[panel], len(m.items(panel))-1))
	}
}

func (m *Model) setFocus(panel Panel) {
	if panel < Podcasts || panel > PlayerPanel {
		return
	}
	m.focus = panel
	if panel == Podcasts || panel == Episodes {
		m.lastVisible = panel
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case animationTick:
		if msg.generation != m.animation.generation || !m.animation.running {
			return m, nil
		}
		if m.animation.ctx.Err() != nil {
			return m.SyncAnimation()
		}
		m.animation.frame = (m.animation.frame + 1) % 1024
		return m.scheduleAnimation()
	case progressTick:
		if msg.generation != m.progress.generation || !m.progress.running || m.progress.ctx == nil || m.progress.ctx.Err() != nil {
			return m, nil
		}
		m.progress.running = false
		return m.SyncProgress()
	case tea.WindowSizeMsg:
		m.width, m.height = max(0, msg.Width), max(0, msg.Height)
		m.scrollDetails(0)
		var animation, progress tea.Cmd
		m, animation = m.SyncAnimation()
		m, progress = m.SyncProgress()
		return m, tea.Batch(animation, progress)
	case Data:
		// Keep the selected identity as asynchronous data replaces list contents.
		for panel := Podcasts; panel <= Queue; panel++ {
			old := m.items(panel)
			if len(old) == 0 {
				continue
			}
			id := old[m.selected[panel]].ID
			updated := m
			updated.data = msg
			for i, item := range updated.items(panel) {
				if item.ID == id {
					m.selected[panel] = i
					break
				}
			}
		}
		m.data = msg
		m.clamp()
	case tea.PasteMsg:
		if m.editing {
			m.filters[m.focus] += msg.Content
			m.clamp()
		}
	case tea.KeyPressMsg:
		key := msg.String()
		_ = key
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.editing {
			switch key {
			case "enter":
				m.editing = false
			case "esc":
				m.filters[m.focus], m.editing = m.previous, false
			case "backspace":
				runes := []rune(m.filters[m.focus])
				if len(runes) > 0 {
					m.filters[m.focus] = string(runes[:len(runes)-1])
				}
			case "ctrl+u":
				m.filters[m.focus] = ""
			default:
				m.filters[m.focus] += msg.Text
			}
			m.clamp()
			return m, nil
		}
		if key == "q" {
			return m, tea.Quit
		}
		if m.settings {
			switch key {
			case "j", "down":
				if m.settingsData.Selected < len(m.settingsData.Themes)-1 {
					m.settingsData.Selected++
				}
			case "k", "up":
				if m.settingsData.Selected > 0 {
					m.settingsData.Selected--
				}
			case "enter":
				if len(m.settingsData.Themes) > 0 && m.actions != nil {
					chosen := m.settingsData.Themes[m.settingsData.Selected]
					m.settingsData.ActiveTheme = chosen
					return m, m.actions("theme:" + chosen)
				}
			case "esc", "e":
				m.settings = false
			}
			return m, nil
		}
		if m.help {
			if key == "esc" || key == "?" {
				m.help = false
			}
			return m, nil
		}
		if m.data.Dialog != nil {
			return m, nil
		}
		if m.focus == PlayerPanel && (key == "h" || key == "l" || key == "," || key == "." || key == "<" || key == ">") {
			if !m.data.Player.CanSeek || m.actions == nil {
				return m, nil
			}
			command := "seek-forward"
			if key == "h" || key == "," || key == "<" {
				command = "seek-backward"
			}
			return m, m.actions(command)
		}
		switch key {
		case "left", "right":
			if m.focus == PlayerPanel {
				step := 1
				if key == "left" {
					step = -1
				}
				m.playerButton = (m.playerButton + step + 3) % 3
			}
		case "enter":
			if m.focus == PlayerPanel {
				return m, m.activatePlayer()
			}
		case "pgdown", "pgup", "ctrl+d", "ctrl+u":
			if m.focus == Details {
				_, height := m.detailsSize()
				delta := max(1, height-3)
				if key == "ctrl+d" || key == "ctrl+u" {
					delta = max(1, delta/2)
				}
				if key == "pgup" || key == "ctrl+u" {
					delta = -delta
				}
				m.scrollDetails(delta)
			}
		case "space", "s", "n":
			if m.actions != nil {
				command := map[string]string{"space": "toggle", "s": "stop", "n": "next"}[key]
				return m, m.actions(command)
			}
		case "?":
			m.help = true
		case "e":
			m.settings = true
		case ",", "<":
			if m.data.Player.CanSeek && m.actions != nil {
				cmd := m.actions("seek-backward")
				return m, cmd
			}
			return m, nil
		case ".", ">":
			if m.data.Player.CanSeek && m.actions != nil {
				return m, m.actions("seek-forward")
			}
		case "h":
			if m.focus < Details {
				m.colOffset[m.focus] = max(0, m.colOffset[m.focus]-4)
			}
			return m, nil
		case "l":
			if m.focus < Details {
				m.colOffset[m.focus] += 4
			}
			return m, nil
		case "tab":
			m.setFocus((m.focus + 1) % (PlayerPanel + 1))
		case "shift+tab":
			m.setFocus((m.focus + PlayerPanel) % (PlayerPanel + 1))
		case "1", "2", "3", "4", "5":
			m.setFocus(Panel(key[0] - '1'))
		case "esc":
			if m.compact() && m.focus >= Queue {
				m.setFocus(m.lastVisible)
			} else if m.focus < Details {
				m.filters[m.focus] = ""
			}
		case "/":
			if m.focus < Details {
				m.editing, m.previous = true, m.filters[m.focus]
			}
		case "j", "down", "k", "up", "g", "G", "home", "end":
			if m.focus == Details {
				switch key {
				case "j", "down":
					m.scrollDetails(1)
				case "k", "up":
					m.scrollDetails(-1)
				case "g", "home":
					m.detailScroll = 0
				default:
					width, _ := m.detailsSize()
					m.scrollDetails(len(m.detailRows(width)))
				}
			} else if m.focus < Details {
				switch key {
				case "j", "down":
					m.selected[m.focus]++
				case "k", "up":
					m.selected[m.focus]--
				case "g", "home":
					m.selected[m.focus] = 0
				default:
					m.selected[m.focus] = len(m.items(m.focus)) - 1
				}
				m.clamp()
			}
		}
	}
	return m, nil
}
