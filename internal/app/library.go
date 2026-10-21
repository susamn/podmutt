package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"podmutt/internal/artwork"
	"podmutt/internal/discovery"
	"podmutt/internal/feed"
	"podmutt/internal/library"
	"podmutt/internal/playback"
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
	version   uint64
	snapshot  library.Snapshot
	details   *store.Episode
	detailKey string
	err       error
}
type libraryEdited struct {
	id      int64
	added   bool
	state   *playback.State
	episode *store.Episode
	err     error
}
type refreshUpdate struct {
	result feed.Result
	ok     bool
}
type libraryShell struct {
	art    artState
	browse browseState
	shell
	signals                  chan os.Signal
	service                  *library.Service
	refresher                *feed.Refresher
	results                  <-chan feed.Result
	snapshot                 library.Snapshot
	rows                     []store.Episode
	libraryID, wantLibraryID string
	order                    library.Order
	version                  uint64
	loadCancel               context.CancelFunc
	notice, input            string
	adding, mutating         bool
	refreshErrors            map[int64]string
	lastPlayingID            int64
	lastCompleted            bool
	confirming               *queueConfirmation
	queueEditing             bool
	queueUndoPending         bool
	detailTarget             *store.Episode
	detailFrom               ui.Panel
	startupDone              bool
	queueSelectID            string
	onboarding               bool
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
	model := libraryShell{art: newArt(), shell: newShell(s), service: service, refresher: refresher, snapshot: snapshot, libraryID: allDestination, wantLibraryID: allDestination,
		notice: "Cached library", refreshErrors: make(map[int64]string)}
	appleDir := discovery.NewApple(nil)
	appleDir.SetCountry(s.config.Country)
	model.browse.directory = appleDir
	model.browse.indexDirectory = discovery.NewIndex(os.Getenv(discovery.IndexKeyEnv), os.Getenv(discovery.IndexSecretEnv), nil)
	state, err := s.playback.Snapshot(s.ctx)
	if err != nil {
		return libraryShell{}, err
	}
	model.shell = model.shell.withPlayback(state)
	model.lastPlayingID, model.lastCompleted = 0, false
	model.browse.active = model.subscribedCount() == 0 && len(model.allEpisodes()) == 0
	if model.browse.active {
		model.Model = model.Model.WithFocus(ui.Podcasts)
	} else {
		model.Model = model.Model.WithFocus(ui.Episodes)
	}
	model.renderLibrary()
	model.alignStartup()
	model.renderDetails()
	model.onboarding = !s.paths.Onboarded()
	if model.onboarding {
		model.renderOnboarding()
	}
	// Constructor discards commands; arm presentation timers from Init messages,
	// not from the synthetic initial snapshot.
	model.Model = model.Model.WithProgress(s.ctx, nil, nil)
	signals := make(chan os.Signal, 1)
	theme.NotifyReload(signals)
	model.signals = signals
	return model, nil
}
func (m libraryShell) Init() tea.Cmd {
	return tea.Batch(m.shell.Init(), m.waitRefresh(), m.waitSignal(), func() tea.Msg { return discoverStartup{} }, func() tea.Msg { return libraryStartup{} }, func() tea.Msg { return artProbe{} })
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
			return work(m.s.ctx, "Reloading theme…", func(context.Context) tea.Msg {
				pal, themePath := theme.Load(m.s.paths.ConfigDir(), m.s.config.Theme)
				return themeReloadMsg{palette: pal, themeName: m.s.config.Theme, themePath: themePath}
			})()
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
	version, id, service, repo := m.version, m.libraryFeedID(), m.service, m.s.store
	var target *store.Episode
	detailKey, previewURL := "", ""
	if m.Model.Focus() == ui.Details && m.detailTarget != nil {
		copy := *m.detailTarget
		target, detailKey = &copy, m.detailKey(copy)
		if copy.ID == 0 && m.browse.preview != nil {
			previewURL = m.browse.preview.URL
		}
	}
	return work(ctx, "Loading library…", func(ctx context.Context) tea.Msg {
		snapshot, err := service.Read(ctx, id)
		var details *store.Episode
		if err == nil {
			details = readInspected(ctx, repo, snapshot.Feeds, target, previewURL)
		}
		return libraryLoaded{version: version, snapshot: snapshot, details: details, detailKey: detailKey, err: err}
	})
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
			updated.applyQueueFocus()
			var cmd tea.Cmd
			updated, cmd = updated.syncArt()
			var progress tea.Cmd
			updated.Model, progress = updated.Model.SyncProgress()
			var animation tea.Cmd
			updated.Model, animation = updated.Model.SyncAnimation()
			result = updated
			command = tea.Batch(command, cmd, progress, animation)
		}
	}()
	var cmd tea.Cmd
	rebuild, alignCurrent := false, false
	switch msg := msg.(type) {
	case countryChangedMsg:
		m.s.config.Country = msg.country
		if apple, ok := m.browse.directory.(*discovery.Apple); ok {
			apple.SetCountry(msg.country)
		}
		m.Model = m.Model.WithSettings(m.s.settingsData())
		m.notice = "Country set to " + strings.ToUpper(msg.country)
		if m.browse.active {
			m.browse.loaded = false
			return m, m.searchDirectory("")
		}
		return m, nil
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
	case libraryStartup:
		cmd = m.resumeStartup()
	case discoverStartup:
		if m.browse.active && !m.browse.loaded && !m.browse.loading {
			cmd = m.searchDirectory("")
		}
	case directoryLoaded:
		m.applyDirectory(msg)
	case previewLoaded:
		m.applyPreview(msg)
	case showRatingLoaded:
		if msg.rating > 0 {
			for i := range m.browse.shows {
				if m.browse.shows[i].FeedURL == msg.feedURL {
					m.browse.shows[i].Rating = msg.rating
					m.browse.shows[i].ReviewCount = msg.count
					break
				}
			}
			if m.s.store != nil {
				sourceID, _ := m.s.store.SourceID(m.s.ctx, "apple")
				if sourceID > 0 {
					shows, _ := m.s.store.DiscoveredShows(m.s.ctx, m.s.config.Country, "", 100)
					for _, s := range shows {
						if s.FeedURL == msg.feedURL {
							_ = m.s.store.UpsertShowRating(m.s.ctx, s.ID, sourceID, msg.rating, msg.count)
							break
						}
					}
				}
			}
			m.renderDetails()
		}
		return m, nil
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
			if msg.details != nil && m.detailTarget != nil && m.detailKey(*m.detailTarget) == msg.detailKey {
				m.detailTarget.PositionSeconds, m.detailTarget.Completed = msg.details.PositionSeconds, msg.details.Completed
			}
			if m.browse.localResults {
				cmd = m.searchLibrary(m.browse.localQuery)
			}
			rebuild = true
		}
	case latestQueued:
		if msg.err != nil {
			m.notice = msg.err.Error()
		}
	case queueEdited:
		m.queueEditing = false
		pending := m.queueUndoPending
		m.queueUndoPending = false
		if errors.Is(msg.err, playback.ErrNoQueueUndo) {
			m.notice = "Nothing to undo in Up next"
		} else if msg.err != nil {
			m.notice = msg.err.Error()
		} else if msg.undo {
			m.notice = "Restored to Up next"
		} else {
			m.notice = "Removed from Up next · u undo"
			if pending {
				cmd = m.undoQueueRemoval()
			}
		}
	case playerEdited:
		if msg.state != nil {
			m.shell = m.shell.withPlayback(*msg.state)
			rebuild = true
		}
		if msg.err == nil && len(msg.enqueued) > 0 {
			var ids []string
			for _, id := range msg.enqueued {
				ids = append(ids, strconv.FormatInt(id, 10))
			}
			m.Model = m.Model.WithQueueTail(ids)
			m.queueSelectID = ids[len(ids)-1]
		}
		if msg.cached {
			cmd = m.reload()
		}
		if errors.Is(msg.err, playback.ErrNoEpisode) {
			m.notice = "No current episode · Enter plays the selected episode"
		} else if msg.err != nil {
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
		if msg.state != nil {
			m.shell = m.shell.withPlayback(*msg.state)
		}
		if msg.episode != nil && m.detailTarget != nil && m.detailTarget.ID == msg.episode.ID {
			m.detailTarget.Completed = msg.episode.Completed
		}
		if msg.added {
			m.discardIndexResults()
			m.cancelDirectory()
			m.cancelPreview()
			m.browse.active, m.browse.localResults = false, false
			m.Model = m.Model.ClearFilter(ui.Podcasts).ClearFilter(ui.Episodes).WithFocus(ui.Episodes)
			m.libraryID = strconv.FormatInt(msg.id, 10)
			m.wantLibraryID = m.libraryID
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
		if msg.ok && msg.state.Revision != 0 && msg.state.Revision < m.state.Revision {
			return m, m.shell.wait()
		}
		changed := msg.ok && msg.state.Current != nil && (m.state.Current == nil || m.state.Current.ID != msg.state.Current.ID)
		if msg.ok {
			id, completed := int64(0), false
			if msg.state.Current != nil {
				id, completed = msg.state.Current.ID, msg.state.Current.Completed
			}
			if m.lastPlayingID != id || m.lastCompleted != completed {
				cmd = m.reload()
			}
			started := func(episode *store.Episode) bool { return episode != nil && episode.PositionSeconds > 0 }
			rebuild = m.lastPlayingID != id || m.lastCompleted != completed || started(m.state.Current) != started(msg.state.Current) ||
				currentInQueue(m.state) != currentInQueue(msg.state) ||
				!slices.EqualFunc(m.state.Queue, msg.state.Queue, func(a, b store.Episode) bool { return a.ID == b.ID })
			m.lastPlayingID, m.lastCompleted = id, completed
		}
		updated, wait := m.shell.Update(msg)
		m.shell = updated.(shell)
		alignCurrent = changed
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
		if m.onboarding {
			return m, m.onboardingKey(msg)
		}
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
		key := msg.String()
		focus := m.Model.Focus()
		switch key {
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
			if msg.String() == "esc" && m.Model.HasFilter(focus) {
				return m.delegate(msg)
			}
			if m.browse.loading || m.browse.previewCancel != nil {
				m.cancelDirectory()
				m.cancelPreview()
				m.notice = "Discovery request canceled"
				m.renderLibrary()
			} else if m.Model.Focus() == ui.Details && m.detailTarget != nil {
				m.leaveDetails()
			} else if m.browse.active && m.browse.preview != nil && (focus == ui.Podcasts || focus == ui.Episodes) {
				m.cancelPreview()
				m.Model = m.Model.WithFocus(ui.Podcasts)
				m.renderLibrary()
			} else if m.browse.localResults {
				cmd = m.searchLibrary("")
			} else {
				return m.delegate(msg)
			}
		case "space":
			engine, ctx := m.s.playback, m.s.ctx
			cmd = work(ctx, "Updating playback…", func(ctx context.Context) tea.Msg { return playerEdited{err: engine.Toggle(ctx, 0)} })
		case "i":
			m.openDetails()
		case "a", "A":
			cmd = m.enqueue(msg.String() == "A")
		case "J", "K":
			// Queue order is presentation-only; manual moves are no longer supported.
		case "d", "delete":
			cmd = m.removeQueueItem()
		case "D":
			m.confirmClearQueue()
		case "N":
			if !m.mutating {
				m.adding = true
				m.input = ""
			}
		case "o":
			if focus == ui.Queue {
				m.Model = m.Model.ToggleQueueOrder()
			} else if focus == ui.Podcasts || focus == ui.Episodes {
				m.order = (m.order + 1) % 3
				rebuild = true
			}
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
			if focus == ui.Queue {
				cmd = m.undoQueueRemoval()
			} else if m.browse.active {
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
				cmd = m.syncLibrarySelection()
				rebuild = true
				m.Model = m.Model.WithFocus(ui.Episodes)
				if cmd == nil {
					cmd = m.reload()
				}
			} else if m.Model.Focus() == ui.Queue || m.Model.Focus() == ui.Episodes || m.Model.Focus() == ui.Details {
				cmd = m.playSelected()
			}
		default:
			return m.delegate(msg)
		}
	default:
		return m.delegate(msg)
	}
	if rebuild {
		m.renderLibrary()
		cmd = tea.Batch(cmd, m.syncLibrarySelection())
	} else {
		m.renderStatus()
	}
	if alignCurrent {
		m.selectCurrent()
		m.renderDetails()
	}
	return m, cmd
}
func (m libraryShell) delegate(msg tea.Msg) (tea.Model, tea.Cmd) {
	before, _ := m.Model.Selected(ui.Podcasts)
	origin := m.Model.Focus()
	episode, hasEpisode := m.contextEpisode()
	updated, cmd := m.shell.Update(msg)
	m.shell = updated.(shell)
	if origin != ui.Details && m.Model.Focus() == ui.Details && hasEpisode {
		m.detailTarget, m.detailFrom = &episode, origin
	}
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
	cmd = tea.Batch(cmd, m.syncLibrarySelection())
	m.renderDetails()
	return m, cmd
}
func (m *libraryShell) appendInput(text string) {
	ti := ui.TextInput{Value: m.input, Limit: 4096}
	ti.Append(text)
	m.input = ti.Value
}
func (m *libraryShell) inputKey(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "esc":
		m.adding = false
		m.input = ""
	case "enter":
		m.adding = false
		m.mutating = true
		m.notice = "Adding feed…"
		address, service, ctx, engine := m.input, m.service, m.s.ctx, m.s.playback
		m.input = ""
		return work(ctx, "Adding podcast…", func(ctx context.Context) tea.Msg {
			id, err := service.Add(ctx, address)
			return subscriptionEdited(ctx, engine, id, true, err)
		})
	default:
		ti := ui.TextInput{Value: m.input, Limit: 4096}
		if ti.EditKey(key) {
			m.input = ti.Value
		}
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
			return work(ctx, "Saving subscription…", func(ctx context.Context) tea.Msg {
				return subscriptionEdited(ctx, engine, source.ID, false, service.Subscribe(ctx, source.ID, !source.Subscribed))
			})
		}
	}
	m.notice = "Select a show first"
	return nil
}
func (m *libraryShell) complete() tea.Cmd {
	if m.previewActionContext() {
		m.notice = "Preview only · u subscribe before changing history"
		return nil
	}
	if m.mutating {
		return nil
	}
	episode, ok := m.contextEpisode()
	if !ok || (m.Model.Focus() != ui.Episodes && m.Model.Focus() != ui.Details) || episode.ID <= 0 {
		m.notice = "Select an episode first"
		return nil
	}
	m.mutating = true
	engine, ctx := m.s.playback, m.s.ctx
	return work(ctx, "Saving listening history…", func(ctx context.Context) tea.Msg {
		if err := engine.MarkCompleted(ctx, episode.ID, !episode.Completed); err != nil {
			return libraryEdited{err: err}
		}
		episode.Completed = !episode.Completed
		state, err := engine.Snapshot(ctx)
		return libraryEdited{state: &state, episode: &episode, err: err}
	})
}

func (m *libraryShell) renderLibrary() {
	if m.browse.active {
		m.renderDiscovery()
		return
	}
	m.snapshot = m.snapshot.WithCurrent(m.state.Current)
	view := m.libraryView()
	label := view.String()
	if view == library.AllView {
		label = "All episodes"
	}
	if view == library.ShowView {
		for _, source := range m.snapshot.Feeds {
			if source.ID == m.libraryFeedID() {
				label = "Show: " + library.PlainText(source.Title)
				break
			}
		}
	}
	if m.browse.localResults {
		label = "Library search: " + m.browse.localQuery
	}
	data := ui.Data{PodcastLabel: "My library · cached", EpisodeLabel: label + " · " + m.order.String()}
	switch view {
	case library.NewView:
		data.EpisodeEmpty = "No unstarted subscribed episodes"
	case library.ContinueView:
		data.EpisodeEmpty = "Nothing to resume · play an episode first"
	case library.AllView:
		data.EpisodeEmpty = "No subscribed episodes · f find a podcast"
	}
	if m.browse.localResults {
		data.EpisodeEmpty = "No matching cached episodes"
	}
	if !m.browse.localResults {
		for _, destination := range libraryDestinations {
			data.Podcasts = append(data.Podcasts, ui.Item{ID: destination.id, Title: destination.view.String(), Destination: true})
		}
	}
	for _, source := range m.snapshot.Feeds {
		if !source.Subscribed || !m.matchesShow(source) {
			continue
		}
		title := library.PlainText(source.Title)
		if title == "" {
			title = source.URL
		}
		data.Podcasts = append(data.Podcasts, ui.Item{ID: strconv.FormatInt(source.ID, 10), Title: title, Channel: title})
	}
	episodes := m.snapshot
	if view == library.AllView {
		episodes.All = m.allEpisodes()
	}
	if m.browse.localResults {
		episodes.Show = m.browse.hits
		view = library.ShowView
	}
	m.rows = library.Episodes(episodes, view, m.order)
	queuedSet := make(map[int64]bool)
	for _, qItem := range m.state.Queue {
		queuedSet[qItem.ID] = true
	}
	if currentInQueue(m.state) {
		queuedSet[m.state.Current.ID] = true
	}
	for _, episode := range m.rows {
		item := episodeItem(episode, queuedSet[episode.ID])
		item.Current = m.state.Current != nil && m.state.Current.ID == episode.ID
		data.Episodes = append(data.Episodes, item)
	}
	m.Model = m.Model.WithLibrary(data)
	if m.wantLibraryID != "" {
		m.Model = m.Model.Select(ui.Podcasts, m.wantLibraryID)
		if item, _ := m.Model.Selected(ui.Podcasts); item.ID == m.wantLibraryID {
			m.wantLibraryID = ""
		} else {
			for _, source := range m.snapshot.Feeds {
				if strconv.FormatInt(source.ID, 10) == m.wantLibraryID && !source.Subscribed {
					m.wantLibraryID = ""
					break
				}
			}
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

func (m *libraryShell) renderOnboarding() {
	m.Model = m.Model.WithDialog(&ui.Dialog{
		Title: "Welcome to podmutt",
		Body: "A keyboard-driven podcast client for Linux and macOS.\n\n" +
			"Quick Start:\n" +
			"  • f         Find podcasts by name or paste an RSS/web URL\n" +
			"  • b         Toggle between My Library and Discover (Top 25)\n" +
			"  • Enter     Play selected episode (or preview show)\n" +
			"  • Space     Pause / Resume playback from any pane\n" +
			"  • , / .     Seek backward / forward 15 seconds\n" +
			"  • a / d / u Enqueue episode / remove / undo removal\n" +
			"  • Tab / 1-5 Switch panels (Podcasts, Episodes, Queue, Details, Player)\n" +
			"  • ?         Open full keybinding cheat sheet",
		Hint: "Enter or Esc to start · q quit",
	})
}

func (m *libraryShell) onboardingKey(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "q", "ctrl+c":
		return tea.Quit
	case "enter", "esc", "space":
		m.onboarding = false
		m.Model = m.Model.WithDialog(nil)
		_ = m.s.paths.SetOnboarded()
		return nil
	default:
		return nil
	}
}
