package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"podmutt/internal/artwork"
	"podmutt/internal/discovery"
	"podmutt/internal/feed"
	"podmutt/internal/library"
	"podmutt/internal/store"
	"podmutt/internal/theme"
	"podmutt/internal/ui"
)

type themeReloadMsg struct {
	palette   theme.Palette
	themeName string
	themePath string
}

type libraryLoaded struct {
	version  uint64
	snapshot library.Snapshot
	err      error
}
type libraryEdited struct {
	id    int64
	added bool
	err   error
}
type refreshUpdate struct {
	result feed.Result
	ok     bool
}
type libraryShell struct {
	art    artState
	browse browseState
	shell
	signals            chan os.Signal
	service            *library.Service
	refresher          *feed.Refresher
	results            <-chan feed.Result
	snapshot           library.Snapshot
	rows               []store.Episode
	feedID, wantFeedID int64
	newOnly            bool
	order              library.Order
	version            uint64
	loadCancel         context.CancelFunc
	notice, input      string
	adding, mutating   bool
	refreshErrors      map[int64]string
	lastPlayingID      int64
	lastCompleted      bool
	confirming         *queueConfirmation
	detailTarget       *store.Episode
}

func runLibrary(s *session, input io.Reader, output io.Writer) error {
	lifetime, cancel := context.WithCancel(s.ctx)
	defer cancel()
	local := *s
	local.ctx = lifetime
	refresher := feed.New(s.store, nil, time.Duration(s.config.RefreshMinutes)*time.Minute)
	if err := s.playback.QueueLatest(lifetime, 0); err != nil {
		return err
	}
	model, err := newLibrary(&local, refresher)
	if err != nil {
		return err
	}
	results, err := refresher.Start(lifetime)
	if err != nil {
		return err
	}
	defer func() {
		cancel()
		for range results {
		}
	}()
	model.results = results
	defer func() {
		if model.art.possible {
			fmt.Fprint(output, artwork.KittyDelete(model.art.id))
		}
	}()
	_, err = tea.NewProgram(model, tea.WithInput(input), tea.WithOutput(output)).Run()
	return err
}
func newLibrary(s *session, refresher *feed.Refresher) (libraryShell, error) {
	service := library.New(s.store, refresher)
	snapshot, err := service.Read(s.ctx, 0)
	if err != nil {
		return libraryShell{}, err
	}
	model := libraryShell{art: newArt(), shell: newShell(s), service: service, refresher: refresher, snapshot: snapshot, newOnly: true,
		notice: "Cached library", refreshErrors: make(map[int64]string)}
	model.browse.directory = discovery.NewApple(nil)
	model.browse.indexDirectory = discovery.NewIndex(os.Getenv(discovery.IndexKeyEnv), os.Getenv(discovery.IndexSecretEnv), nil)
	model.browse.active = model.subscribedCount() == 0
	model.renderLibrary()
	state, err := s.playback.Snapshot(s.ctx)
	if err != nil {
		return libraryShell{}, err
	}
	updated, _ := model.shell.Update(playbackUpdate{state, true})
	model.shell = updated.(shell)
	// Constructor discards commands; arm presentation timers from Init messages,
	// not from the synthetic initial snapshot.
	model.Model = model.Model.WithProgress(s.ctx, nil, nil)
	model.feedID = model.selectedFeed()
	signals := make(chan os.Signal, 1)
	theme.NotifyReload(signals)
	model.signals = signals
	return model, nil
}
func (m libraryShell) Init() tea.Cmd {
	return tea.Batch(m.shell.Init(), m.waitRefresh(), m.waitSignal(), func() tea.Msg { return discoverStartup{} }, func() tea.Msg { return artProbe{} })
}
func (m libraryShell) waitSignal() tea.Cmd {
	if m.signals == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case <-m.s.ctx.Done():
			return nil
		case <-m.signals:
			pal, themePath := theme.Load(m.s.paths.ConfigDir(), m.s.config.Theme)
			return themeReloadMsg{palette: pal, themeName: m.s.config.Theme, themePath: themePath}
		}
	}
}
func (m libraryShell) waitRefresh() tea.Cmd {
	return func() tea.Msg {
		select {
		case result, ok := <-m.results:
			return refreshUpdate{result, ok}
		case <-m.s.ctx.Done():
			return refreshUpdate{}
		}
	}
}
func (m *libraryShell) reload() tea.Cmd {
	if m.loadCancel != nil {
		m.loadCancel()
	}
	ctx, cancel := context.WithCancel(m.s.ctx)
	m.loadCancel = cancel
	m.version++
	version, id, service := m.version, m.feedID, m.service
	return func() tea.Msg { snapshot, err := service.Read(ctx, id); return libraryLoaded{version, snapshot, err} }
}
func (m libraryShell) selectedFeed() int64 {
	item, _ := m.Model.Selected(ui.Podcasts)
	id, _ := strconv.ParseInt(item.ID, 10, 64)
	return id
}
func (m libraryShell) selectedEpisode() (store.Episode, bool) {
	if m.browse.active {
		return m.previewEpisode()
	}
	item, ok := m.Model.Selected(ui.Episodes)
	if !ok {
		return store.Episode{}, false
	}
	for _, episode := range m.rows {
		if item.ID == strconv.FormatInt(episode.ID, 10) {
			return episode, true
		}
	}
	return store.Episode{}, false
}

func (m libraryShell) Update(msg tea.Msg) (result tea.Model, command tea.Cmd) {
	defer func() {
		if updated, ok := result.(libraryShell); ok {
			var cmd tea.Cmd
			updated, cmd = updated.syncArt()
			var progress tea.Cmd
			updated.Model, progress = updated.Model.SyncProgress()
			result = updated
			command = tea.Batch(command, cmd, progress)
		}
	}()
	var cmd tea.Cmd
	rebuild := false
	switch msg := msg.(type) {
	case themeReloadMsg:
		m.s.theme = msg.palette
		if msg.themeName != "" {
			m.s.config.Theme = msg.themeName
		}
		m.Model = m.Model.WithTheme(msg.palette).WithSettings(m.s.settingsData())
		return m, m.waitSignal()
	case artProbe:
		cmd = m.probeArt()
	case artProbeTimeout:
		if !m.art.kitty {
			m.art.probeExpired = true
			m.art.probed = true
		}
	case tea.TerminalVersionMsg:
		if m.art.possible && !m.art.probeExpired {
			m.art.versionOK = artwork.KittyVersion(msg.Name)
			m.art.kitty = m.art.versionOK && m.art.graphicsOK
		}
	case uv.KittyGraphicsEvent:
		if m.art.possible && !m.art.probed && uint32(msg.Options.ID) == m.art.id {
			m.art.probed = true
			m.art.graphicsOK = string(msg.Payload) == "OK"
			m.art.kitty = m.art.graphicsOK && m.art.versionOK
		}
	case artAcknowledged:
		m.art.inFlight = false
	case artLoaded:
		if msg.generation != m.art.generation {
			return m, nil
		}
		if m.art.cancel != nil {
			m.art.cancel()
			m.art.cancel = nil
		}
		m.art.complete = true
		m.art.image = msg.img
		if msg.err != nil {
			m.art.image = nil
		}
	case discoverStartup:
		if m.browse.active && !m.browse.loaded && !m.browse.loading {
			cmd = m.searchDirectory("")
		}
	case directoryLoaded:
		m.applyDirectory(msg)
	case previewLoaded:
		m.applyPreview(msg)
	case libraryLoaded:
		if msg.version != m.version {
			return m, nil
		}
		if m.loadCancel != nil {
			m.loadCancel()
			m.loadCancel = nil
		}
		if msg.err != nil {
			m.notice = msg.err.Error()
		} else {
			m.snapshot = msg.snapshot
			if m.browse.localResults {
				cmd = m.searchLibrary(m.browse.localQuery)
			}
			rebuild = true
		}
	case latestQueued:
		if msg.err != nil {
			m.notice = msg.err.Error()
		}
	case playerEdited:
		if msg.err != nil {
			m.notice = msg.err.Error()
		} else {
			m.notice = "Playback/queue updated"
		}
	case libraryEdited:
		m.mutating = false
		if msg.err != nil {
			m.notice = msg.err.Error()
			break
		}
		m.notice = "Library updated"
		if msg.added {
			m.discardIndexResults()
			m.cancelDirectory()
			m.cancelPreview()
			m.browse.active, m.browse.localResults = false, false
			m.Model = m.Model.ClearFilter(ui.Podcasts).ClearFilter(ui.Episodes).WithFocus(ui.Episodes)
			m.feedID, m.wantFeedID = msg.id, msg.id
			m.newOnly = false
			m.snapshot.Show = nil
			rebuild = true
			m.refresher.RefreshNow()
		}
		cmd = m.reload()
	case refreshUpdate:
		if !msg.ok {
			return m, nil
		}
		if msg.result.Error != nil {
			m.refreshErrors[msg.result.FeedID] = msg.result.Error.Error()
		} else {
			delete(m.refreshErrors, msg.result.FeedID)
		}
		m.notice = "Refresh update"
		cmd = tea.Batch(m.waitRefresh(), m.reload())
		if msg.result.Error == nil {
			cmd = tea.Batch(cmd, m.queueLatest(msg.result.FeedID))
		}
	case playbackUpdate:
		if msg.ok {
			id, completed := int64(0), false
			if msg.state.Current != nil {
				id, completed = msg.state.Current.ID, msg.state.Current.Completed
			}
			if m.lastPlayingID != id || m.lastCompleted != completed {
				cmd = m.reload()
			}
			m.lastPlayingID, m.lastCompleted = id, completed
		}
		updated, wait := m.shell.Update(msg)
		m.shell = updated.(shell)
		m.renderDetails()
		cmd = tea.Batch(wait, cmd)
	case tea.PasteMsg:
		if m.browse.searching {
			m.appendSearch(msg.Content)
			break
		}
		if m.adding {
			m.appendInput(msg.Content)
			break
		}
		return m.delegate(msg)
	case tea.KeyPressMsg:
		if m.confirming != nil {
			cmd = m.confirmationKey(msg)
			break
		}
		if m.adding {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			cmd = m.inputKey(msg)
			break
		}
		if m.browse.searching {
			cmd = m.searchKey(msg)
			break
		}
		if m.Model.Busy() {
			return m.delegate(msg)
		}
		switch msg.String() {
		case "b":
			cmd = m.switchBrowse()
		case "f":
			m.openSearch()
		case "U":
			m.openURL()
		case "t":
			m.Model = m.Model.ClearFilter(ui.Podcasts).ClearFilter(ui.Episodes).Reset(ui.Podcasts).Reset(ui.Episodes).WithFocus(ui.Podcasts)
			cmd = m.searchDirectory("")
		case "esc":
			if m.browse.loading || m.browse.previewCancel != nil {
				m.cancelDirectory()
				m.cancelPreview()
				m.notice = "Discovery request canceled"
				m.renderLibrary()
			} else if m.browse.active && m.browse.preview != nil {
				m.cancelPreview()
				m.Model = m.Model.WithFocus(ui.Podcasts)
				m.renderLibrary()
			} else if m.browse.localResults {
				cmd = m.searchLibrary("")
			} else {
				return m.delegate(msg)
			}
		case "space":
			cmd = m.playSelected()
		case "a", "A":
			cmd = m.enqueue(msg.String() == "A")
		case "J", "K":
			step := 1
			if msg.String() == "K" {
				step = -1
			}
			cmd = m.moveQueue(step)
		case "d", "D":
			m.confirmQueue(msg.String() == "D")
		case "N":
			if !m.mutating {
				m.adding = true
				m.input = ""
			}
		case "v":
			m.newOnly = !m.newOnly
			rebuild = true
			cmd = m.reload()
		case "o":
			m.order = (m.order + 1) % 3
			rebuild = true
		case "R":
			if m.browse.active {
				cmd = m.searchDirectory(m.browse.query)
				break
			}
			m.notice = "No subscribed feeds to refresh"
			for _, source := range m.snapshot.Feeds {
				if source.Subscribed {
					m.notice = "Refreshing…"
					m.refresher.RefreshNow()
					break
				}
			}
		case "u":
			if m.browse.active {
				cmd = m.subscribePreview()
			} else {
				cmd = m.subscribe()
			}
		case "c":
			cmd = m.complete()
		case "enter":
			if m.Model.Focus() == ui.PlayerPanel {
				return m.delegate(msg)
			}
			if m.Model.Focus() == ui.Podcasts {
				if m.browse.active {
					cmd = m.previewShow()
					m.renderLibrary()
					break
				}
				if _, ok := m.Model.Selected(ui.Podcasts); !ok {
					m.notice = "Select a show or focus Episodes with 2"
					break
				}
				if m.browse.localResults {
					m.selectLibraryShow()
				}
				m.newOnly = false
				rebuild = true
				m.Model = m.Model.WithFocus(ui.Episodes)
				cmd = m.reload()
			} else if m.Model.Focus() == ui.Queue {
				if episode, ok := m.contextEpisode(); ok {
					engine, ctx, id := m.s.playback, m.s.ctx, episode.ID
					cmd = func() tea.Msg { return playerEdited{engine.PlayEpisode(ctx, id)} }
				}
			} else if m.Model.Focus() == ui.Episodes {
				if episode, ok := m.contextEpisode(); ok {
					m.detailTarget = &episode
					m.Model = m.Model.WithFocus(ui.Details)
					m.renderDetails()
				}
			}
		default:
			return m.delegate(msg)
		}
	default:
		return m.delegate(msg)
	}
	if rebuild {
		m.renderLibrary()
		if !m.browse.active && !m.browse.localResults && m.wantFeedID == 0 {
			if id := m.selectedFeed(); id != m.feedID {
				m.feedID = id
				m.snapshot.Show = nil
				if !m.newOnly {
					m.renderLibrary()
					cmd = tea.Batch(cmd, m.reload())
				}
			}
		}
	} else {
		m.renderStatus()
	}
	return m, cmd
}
func (m libraryShell) delegate(msg tea.Msg) (tea.Model, tea.Cmd) {
	before, _ := m.Model.Selected(ui.Podcasts)
	updated, cmd := m.shell.Update(msg)
	m.shell = updated.(shell)
	if m.browse.active {
		after, _ := m.Model.Selected(ui.Podcasts)
		if before.ID != after.ID {
			m.cancelPreview()
			m.renderLibrary()
		} else {
			m.renderDetails()
		}
		return m, cmd
	}
	if m.browse.localResults {
		m.renderDetails()
		return m, cmd
	}
	if id := m.selectedFeed(); m.wantFeedID == 0 && id != m.feedID {
		m.feedID = id
		m.snapshot.Show = nil
		if !m.newOnly {
			m.Model = m.Model.Reset(ui.Episodes)
		}
		cmd = tea.Batch(cmd, m.reload())
		m.renderLibrary()
	} else {
		m.renderDetails()
	}
	return m, cmd
}
func (m *libraryShell) appendInput(text string) {
	runes := []rune(m.input + text)
	m.input = string(runes[:min(len(runes), 4096)])
}
func (m *libraryShell) inputKey(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "esc":
		m.adding = false
		m.input = ""
	case "ctrl+u":
		m.input = ""
	case "backspace":
		runes := []rune(m.input)
		m.input = string(runes[:max(0, len(runes)-1)])
	case "enter":
		m.adding = false
		m.mutating = true
		m.notice = "Adding feed…"
		address, service, ctx, engine := m.input, m.service, m.s.ctx, m.s.playback
		m.input = ""
		return func() tea.Msg {
			id, err := service.Add(ctx, address)
			return subscriptionEdited(ctx, engine, id, true, err)
		}
	default:
		m.appendInput(key.Text)
	}
	return nil
}
func (m *libraryShell) subscribe() tea.Cmd {
	if m.mutating {
		return nil
	}
	for _, source := range m.snapshot.Feeds {
		if source.ID == m.selectedFeed() {
			m.mutating = true
			service, ctx, engine := m.service, m.s.ctx, m.s.playback
			return func() tea.Msg {
				return subscriptionEdited(ctx, engine, source.ID, false, service.Subscribe(ctx, source.ID, !source.Subscribed))
			}
		}
	}
	m.notice = "Select a show first"
	return nil
}
func (m *libraryShell) complete() tea.Cmd {
	if m.browse.active && m.Model.Focus() == ui.Episodes {
		m.notice = "Preview only · u subscribe before changing history"
		return nil
	}
	if m.mutating {
		return nil
	}
	episode, ok := m.selectedEpisode()
	if !ok || m.Model.Focus() != ui.Episodes || episode.ID <= 0 {
		m.notice = "Select an episode first"
		return nil
	}
	m.mutating = true
	engine, ctx := m.s.playback, m.s.ctx
	return func() tea.Msg { return libraryEdited{err: engine.MarkCompleted(ctx, episode.ID, !episode.Completed)} }
}

func (m *libraryShell) renderLibrary() {
	if m.browse.active {
		m.renderDiscovery()
		return
	}
	label := "Show"
	if m.newOnly {
		label = "New (uncompleted)"
	}
	if m.browse.localResults {
		label = "Library search: " + m.browse.localQuery
	}
	data := ui.Data{PodcastHint: discoveryActions, PodcastLabel: "My subscriptions · cached", EpisodeLabel: label + " · " + m.order.String(), ContextHint: "f Search · U RSS/website · b Discover · / filter · N add/subscribe · u subscribe · ? help · q quit"}
	for _, source := range m.snapshot.Feeds {
		if !source.Subscribed || !m.matchesShow(source) {
			continue
		}
		marker := "[-] "
		if source.Subscribed {
			marker = "[+] "
		}
		title := library.PlainText(source.Title)
		if title == "" {
			title = source.URL
		}
		data.Podcasts = append(data.Podcasts, ui.Item{ID: strconv.FormatInt(source.ID, 10), Title: marker + title, Channel: title})
	}
	episodes := m.snapshot
	if m.browse.localResults {
		episodes.Show = m.browse.hits
	}
	m.rows = library.Episodes(episodes, m.newOnly && !m.browse.localResults, m.order)
	queuedSet := make(map[int64]bool)
	if m.state.Current != nil {
		queuedSet[m.state.Current.ID] = true
	}
	for _, qItem := range m.state.Queue {
		queuedSet[qItem.ID] = true
	}
	for _, episode := range m.rows {
		marker := "[ ] "
		if episode.Completed {
			marker = "[x] "
		}
		title := library.PlainText(episode.Title)
		if title == "" {
			title = "Untitled episode"
		}
		channel := library.PlainText(episode.FeedTitle)
		dateStr := ""
		if episode.PublishedAt.Unix() > 0 {
			dateStr = episode.PublishedAt.Format("2006-01-02")
		}
		durationStr := "--:--"
		if episode.DurationSeconds > 0 {
			durationStr = clock(float64(episode.DurationSeconds))
		}
		data.Episodes = append(data.Episodes, ui.Item{
			ID:         strconv.FormatInt(episode.ID, 10),
			Title:      marker + title,
			PlainTitle: title,
			Channel:    channel,
			Date:       dateStr,
			Duration:   durationStr,
			Queued:     queuedSet[episode.ID],
			Completed:  episode.Completed,
		})
	}
	m.Model = m.Model.WithLibrary(data)
	if m.wantFeedID != 0 {
		m.Model = m.Model.Select(ui.Podcasts, strconv.FormatInt(m.wantFeedID, 10))
		if m.selectedFeed() == m.wantFeedID {
			m.wantFeedID = 0
		}
	}
	m.renderDetails()
	m.renderStatus()
}

func (m *libraryShell) renderStatus() {
	notice, prompt := m.notice, ""
	var errorID int64
	var warning string
	for id, text := range m.refreshErrors {
		if warning == "" || id < errorID {
			errorID, warning = id, text
		}
	}
	if warning != "" {
		notice = "Refresh failed: " + warning + " · " + notice
	}
	if m.adding {
		prompt = "URL: " + m.input
	}
	m.Model = m.Model.WithStatus(notice, prompt)
}

func (m *libraryShell) renderDetails() {
	if m.browse.active && m.Model.Focus() != ui.Queue && !(m.Model.Focus() == ui.Details && m.detailTarget != nil && m.detailTarget.ID > 0) {
		if m.Model.Focus() != ui.Details {
			m.detailTarget = nil
		}
		m.renderPreviewDetails()
		return
	}
	if m.Model.Focus() != ui.Details {
		m.detailTarget = nil
	}
	details := ""
	episode, ok := m.contextEpisode()
	if m.Model.Focus() == ui.Details && m.detailTarget != nil {
		episode, ok = *m.detailTarget, true
	}
	if ok {
		show := ""
		for _, source := range m.snapshot.Feeds {
			if source.ID == episode.FeedID {
				show = library.PlainText(source.Title)
				break
			}
		}
		published := "unknown date"
		if !episode.PublishedAt.IsZero() {
			published = episode.PublishedAt.Format("2006-01-02")
		}
		details = fmt.Sprintf("%s\n%s · %s · %d:%02d\nCompleted: %t\n\n%s", library.PlainText(episode.Title), show, published,
			episode.DurationSeconds/60, episode.DurationSeconds%60, episode.Completed, library.PlainText(episode.Description))
	}
	m.Model = m.Model.WithDetails(details)
}
