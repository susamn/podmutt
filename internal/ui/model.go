// Package ui contains the terminal model, independent of feeds, storage, and mpv.
package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

type Panel int

const (
	Podcasts Panel = iota
	Episodes
	Queue
	Details
)

type Item struct{ ID, Title string }

type Data struct {
	Podcasts, Episodes, Queue    []Item
	Details, Player              string
	EpisodeLabel, Notice, Prompt string
}

type Model struct {
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
}

func New(data Data) Model     { return Model{data: data} }
func (m Model) Init() tea.Cmd { return nil }

func (m Model) WithActions(actions func(string) tea.Cmd) Model { m.actions = actions; return m }
func (m Model) WithPlayer(text string) Model                   { m.data.Player = text; return m }
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
func (m Model) Busy() bool { return m.editing || m.help }
func (m Model) Selected(panel Panel) (Item, bool) {
	items := m.items(panel)
	if len(items) == 0 {
		return Item{}, false
	}
	return items[m.selected[panel]], true
}
func (m Model) Reset(panel Panel) Model {
	if panel >= Podcasts && panel <= Queue {
		m.selected[panel] = 0
	}
	return m
}

// WithLibrary replaces library fields while preserving playback-owned fields.
func (m Model) WithLibrary(data Data) Model {
	data.Player, data.Queue, data.Details = m.data.Player, m.data.Queue, m.data.Details
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
	m.focus = panel
	if panel == Podcasts || panel == Episodes {
		m.lastVisible = panel
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(0, msg.Width), max(0, msg.Height)
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
		if m.help {
			if key == "esc" || key == "?" {
				m.help = false
			}
			return m, nil
		}
		switch key {
		case "pgdown", "pgup":
			if m.focus == Details {
				delta := 10
				if key == "pgup" {
					delta = -10
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
		case "tab", "l":
			m.setFocus((m.focus + 1) % 4)
		case "shift+tab", "h":
			m.setFocus((m.focus + 3) % 4)
		case "1", "2", "3", "4":
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
			if m.focus < Details {
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
