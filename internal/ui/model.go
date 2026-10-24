// Package ui contains the terminal model, independent of feeds, storage, and mpv.
package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/feedback"
	"podmutt/internal/theme"
)

type Panel int

const (
	Podcasts Panel = iota
	Episodes
	Queue
	Details
	Favorites
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
	Destination     bool
	Queued          bool
	Completed       bool
	Chapters        bool
	Current         bool
	PublishedAt     time.Time
	Index           int
}

type Dialog struct {
	TruncateBody  bool     // preserve one explicit detail per row in compact confirmations
	SectionLabels []string // trusted structural body rows, not remote metadata
	ActionLabel   string   // trusted action row, e.g. the favorite command
	Title         string
	Body          string
	InputLabel    string
	InputValue    string
	Hint          string
}

type Data struct {
	Dialog                     *Dialog
	Podcasts, Episodes, Queue  []Item
	Favorites                  []Item
	Details                    string
	EpisodeDetails             *EpisodeDetails
	Player                     Player
	EpisodeLabel, EpisodeEmpty string
	Notice, Prompt             string
	PodcastLabel, ContextHint  string
	PodcastHint                string
	PodcastEmpty               string
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
	Country     string
}

type Model struct {
	message, backgroundMessage feedback.Message
	messageOpen                bool
	messageScroll              int
	version                    string
	theme                      theme.Palette
	settings                   bool
	settingsPending            bool
	settingsData               SettingsData
	chapters                   *ChapterPicker
	icons                      string
	animation                  animation
	activity                   activityView
	progress                   progressClock
	data                       Data
	width, height              int
	focus                      Panel
	lastVisible                Panel
	selected                   [6]int
	colOffset                  [6]int
	filters                    [6]string
	editing                    bool
	previous                   string
	help                       bool
	actions                    func(string) tea.Cmd
	detailScroll               int
	playerButton               int
	decoration                 bool
	episodeOps                 EpisodeActions
	queueChannel               bool
	queueTail                  []string
	artColumns                 int
	artRows                    []string
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
	m.settingsPending = false
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
	return m.editing || m.help || m.settings || m.messageOpen || m.data.Dialog != nil || m.data.Prompt != "" || m.chapters != nil
}
func (m Model) Visible(panel Panel) []Item      { return m.items(panel) }
func (m Model) WithDialog(dialog *Dialog) Model { m.data.Dialog = dialog; return m }
func (m Model) WithPlayback(queue []Item, player Player) Model {
	var tails []string
	for _, id := range m.queueTail {
		for _, item := range queue {
			if item.ID == id {
				tails = append(tails, id)
				break
			}
		}
	}
	m.queueTail = tails
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
func (m Model) isList(panel Panel) bool {
	return panel == Podcasts || panel == Episodes || panel == Queue || panel == Favorites
}

func (m Model) HasFilter(panel Panel) bool {
	return m.isList(panel) && m.filters[panel] != ""
}

func (m Model) ClearFilter(panel Panel) Model {
	if m.isList(panel) {
		m.filters[panel] = ""
		m.clamp()
	}
	return m
}
func (m Model) Reset(panel Panel) Model {
	if m.isList(panel) {
		m.selected[panel] = 0
		m.colOffset[panel] = 0
	}
	return m
}

// WithLibrary replaces library fields while preserving playback-owned fields.
func (m Model) WithLibrary(data Data) Model {
	data.Player, data.Queue, data.Details = m.data.Player, m.data.Queue, m.data.Details
	data.Dialog = m.data.Dialog
	data.EpisodeDetails = m.data.EpisodeDetails
	updated, _ := m.Update(data)
	return updated.(Model)
}
func (m Model) WithStatus(notice, prompt string) Model {
	m.data.Notice, m.data.Prompt = notice, prompt
	return m
}
func (m Model) WithDetails(text string) Model {
	if text != m.data.Details || m.data.EpisodeDetails != nil {
		m.detailScroll = 0
	}
	m.data.EpisodeDetails = nil
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
	case Favorites:
		source = m.data.Favorites
	default:
		return nil
	}
	if panel == Queue {
		source = m.sortedQueue(source)
	}
	query := strings.ToLower(m.filters[panel])
	var items []Item
	for _, item := range source {
		if strings.Contains(strings.ToLower(item.Title+" "+item.Channel), query) {
			items = append(items, item)
		}
	}
	return items
}

func (m *Model) clamp() {
	for _, panel := range []Panel{Podcasts, Episodes, Queue, Favorites} {
		m.selected[panel] = max(0, min(m.selected[panel], len(m.items(panel))-1))
	}
}

func (m *Model) setFocus(panel Panel) {
	if panel < Podcasts || panel > PlayerPanel {
		return
	}
	m.focus = panel
	if panel == Podcasts || panel == Episodes || panel == Favorites {
		m.lastVisible = panel
	}
}

// back leaves supporting panes without changing the selected episode.
func (m *Model) back() {
	if m.focus == Episodes || m.focus == Favorites {
		m.setFocus(Podcasts)
	} else if m.focus >= Queue {
		m.setFocus(m.lastVisible)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case activityTick:
		if msg.generation != m.activity.generation || !m.activity.running {
			return m, nil
		}
		if m.activity.ctx.Err() != nil {
			return m.SyncActivity()
		}
		m.activity.at = time.Now()
		m.activity.frame++
		return m.scheduleActivity()
	case animationTick:
		if msg.generation != m.animation.generation || !m.animation.running {
			return m, nil
		}
		if m.animation.ctx.Err() != nil || !m.animationVisible() {
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
		layoutChanged := msg.Player.Error != m.data.Player.Error || msg.Player.Warning != m.data.Player.Warning
		// Keep the selected identity as asynchronous data replaces list contents.
		for _, panel := range []Panel{Podcasts, Episodes, Queue, Favorites} {
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
		if layoutChanged {
			m.scrollDetails(0)
		}
	case tea.PasteMsg:
		if m.editing {
			ti := TextInput{Value: m.filters[m.focus], Limit: 256}
			ti.Append(msg.Content)
			m.filters[m.focus] = ti.Value
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
			default:
				ti := TextInput{Value: m.filters[m.focus], Limit: 256}
				if ti.EditKey(msg) {
					m.filters[m.focus] = ti.Value
				}
			}
			m.clamp()
			return m, nil
		}
		if key == "q" {
			return m, tea.Quit
		}
		if m.chapters != nil {
			return m, nil // app owns chapter navigation and guarded seeks
		}
		if m.messageOpen {
			switch key {
			case "esc", "!":
				m.messageOpen = false
				m.message = feedback.Message{}
				m.messageScroll = 0
			case "j", "down", "pgdown", "ctrl+d":
				m.scrollFeedback(1)
			case "k", "up", "pgup", "ctrl+u":
				m.scrollFeedback(-1)
			}
			return m, nil
		}
		if m.settings {
			if m.settingsPending && key != "esc" && key != "e" {
				return m, nil
			}
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
					m.settingsPending = true
					return m, m.actions("theme:" + chosen)
				}
			case "right", "left", "C":
				if m.actions != nil {
					m.settingsPending = true
					return m, m.actions("country:cycle")
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
		switch key {
		case "!":
			if m.activeMessage().Summary != "" {
				m.messageOpen = true
				m.messageScroll = 0
			}
			return m, nil
		case "left", "right":
			if m.focus == PlayerPanel {
				step := 1
				if key == "left" {
					step = -1
				}
				m.playerButton = (m.playerButton + step + 3) % 3
			} else if m.isList(m.focus) {
				step := 4
				if key == "left" {
					step = -4
				}
				m.colOffset[m.focus] = max(0, m.colOffset[m.focus]+step)
			}
			return m, nil
		case "enter":
			if m.focus == PlayerPanel {
				return m, m.activatePlayer()
			}
		case "pgdown", "pgup", "ctrl+d", "ctrl+u":
			if m.focus == Details {
				width, height := m.detailsSize()
				delta := max(1, m.detailCapacity(width, height))
				if key == "ctrl+d" || key == "ctrl+u" {
					delta = max(1, delta/2)
				}
				if key == "pgup" || key == "ctrl+u" {
					delta = -delta
				}
				m.scrollDetails(delta)
			} else if m.isList(m.focus) {
				delta := m.listPageSize()
				if key == "ctrl+d" || key == "ctrl+u" {
					delta = max(1, delta/2)
				}
				if key == "pgup" || key == "ctrl+u" {
					delta = -delta
				}
				m.pageList(delta)
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
			if m.isList(m.focus) {
				m.colOffset[m.focus] = max(0, m.colOffset[m.focus]-4)
			}
			return m, nil
		case "l":
			if m.isList(m.focus) {
				m.colOffset[m.focus] += 4
			}
			return m, nil
		case "tab":
			switch m.focus {
			case Podcasts:
				m.setFocus(Episodes)
			case Episodes:
				m.setFocus(Queue)
			case Queue:
				m.setFocus(Favorites)
			case Favorites:
				m.setFocus(Podcasts)
			default:
				m.setFocus(Podcasts)
			}
		case "shift+tab":
			switch m.focus {
			case Podcasts:
				m.setFocus(Favorites)
			case Episodes:
				m.setFocus(Podcasts)
			case Queue:
				m.setFocus(Episodes)
			case Favorites:
				m.setFocus(Queue)
			default:
				m.setFocus(Favorites)
			}
		case "1", "2", "3", "4", "5", "6":
			m.setFocus(Panel(key[0] - '1'))
		case "esc":
			if m.isList(m.focus) && m.filters[m.focus] != "" {
				m.filters[m.focus] = ""
				m.clamp()
			} else {
				m.back()
			}
		case "/":
			if m.isList(m.focus) {
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
			} else if m.isList(m.focus) {
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
