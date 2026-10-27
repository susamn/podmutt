package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/artwork"
	"podmutt/internal/chapters"
	"podmutt/internal/discovery"
	"podmutt/internal/feed"
	"podmutt/internal/feedback"
	"podmutt/internal/httpurl"
	"podmutt/internal/library"
	"podmutt/internal/playback"
	"podmutt/internal/store"
	"podmutt/internal/theme"
	"podmutt/internal/ui"
)

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
type favoriteEdited struct {
	added bool
	err   error
}
type chaptersSynced struct{ err error }

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
	refreshErrors            map[int64]error
	backgroundError          feedback.Message
	lastPlayingID            int64
	lastCompleted            bool
	confirming               *queueConfirmation
	urlDialog                *ui.Dialog
	queueEditing             bool
	queueUndoPending         bool
	detailTarget             *store.Episode
	detailFrom               ui.Panel
	chapterRequest           chapterRequest
	chapterList              []chapters.Chapter
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
		notice: "Cached library", refreshErrors: make(map[int64]error)}
	appleDir := discovery.NewApple(nil)
	appleDir.SetCountry(s.config.Country)
	model.browse.directory = appleDir
	model.browse.country = s.config.Country
	if model.browse.country == "" {
		model.browse.country = "us"
	}
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
	return tea.Batch(m.shell.Init(), m.waitRefresh(), m.waitSignal(), m.waitMidnight(), func() tea.Msg { return discoverStartup{} }, func() tea.Msg { return libraryStartup{} }, func() tea.Msg { return artProbe{} })
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
			return themeSignal{}
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

type midnightTick struct{}

func (m libraryShell) waitMidnight() tea.Cmd {
	ctx := m.s.ctx
	return func() tea.Msg {
		now := currentTime()
		loc := now.Location()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 1, 0, loc)
		delay := nextMidnight.Sub(now)
		if delay <= 0 {
			delay = time.Second
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			return midnightTick{}
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

func (m libraryShell) postUpdate(command tea.Cmd) (tea.Model, tea.Cmd) {
	if chapterPlaybackChanged(m.chapterRequest, m.state) {
		m.closeChapters()
	}
	m.applyQueueFocus()
	var cmd tea.Cmd
	m, cmd = m.syncArt()
	var progress tea.Cmd
	m.Model, progress = m.Model.SyncProgress()
	var animation tea.Cmd
	m.Model, animation = m.Model.SyncAnimation()
	return m, tea.Batch(command, cmd, progress, animation)
}

func (m libraryShell) Update(msg tea.Msg) (result tea.Model, command tea.Cmd) {
	defer func() {
		if updated, ok := result.(libraryShell); ok {
			result, command = updated.postUpdate(command)
		}
	}()

	if updated, cmd, handled := m.updateSettings(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.updateArtwork(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.updateDiscovery(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.updatePlaybackEvents(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.updateLibraryEvents(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.updateInput(msg); handled {
		return updated, cmd
	}
	return m.delegate(msg)
}

func (m libraryShell) updateLibraryEvents(msg tea.Msg) (libraryShell, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case midnightTick:
		m.renderLibrary()
		m.renderDetails()
		return m, m.waitMidnight(), true
	case libraryStartup:
		return m, m.resumeStartup(), true
	case libraryLoaded:
		if msg.version != m.version {
			return m, nil, true
		}
		if m.loadCancel != nil {
			m.loadCancel()
			m.loadCancel = nil
		}
		var cmd tea.Cmd
		if msg.err != nil {
			m.recordBackgroundFailure("Load library", msg.err)
		} else {
			m.clearBackgroundFailure("Load library")
			m.snapshot = msg.snapshot
			if msg.details != nil && m.detailTarget != nil && m.detailKey(*m.detailTarget) == msg.detailKey {
				m.detailTarget.PositionSeconds, m.detailTarget.Completed = msg.details.PositionSeconds, msg.details.Completed
				if m.detailTarget.AudioURL == msg.details.AudioURL {
					m.detailTarget.ChaptersURL, m.detailTarget.ChaptersJSON, m.detailTarget.ChaptersState =
						msg.details.ChaptersURL, msg.details.ChaptersJSON, msg.details.ChaptersState
				}
			}
			if m.browse.localResults {
				cmd = m.searchLibrary(m.browse.localQuery)
			}
			m.renderLibrary()
			cmd = tea.Batch(cmd, m.syncLibrarySelection())
		}
		return m, cmd, true
	case libraryEdited:
		m.mutating = false
		if msg.err != nil {
			m.report("Library update", msg.err, "")
			var fault *feedback.Fault
			var cmd tea.Cmd
			if errors.As(msg.err, &fault) && fault.Code == feedback.Partial {
				cmd = m.reload()
			}
			return m, cmd, true
		}
		m.Model = m.Model.ClearFeedback("Library update")
		m.notice = ""
		if msg.state != nil {
			m.shell = m.shell.withPlayback(*msg.state)
		}
		if msg.episode != nil && m.detailTarget != nil && m.detailTarget.ID == msg.episode.ID {
			m.detailTarget.Completed = msg.episode.Completed
		}
		rebuild := false
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
		cmd := m.reload()
		if rebuild {
			m.renderLibrary()
			cmd = tea.Batch(cmd, m.syncLibrarySelection())
		} else {
			m.renderStatus()
		}
		return m, cmd, true
	case refreshUpdate:
		if !msg.ok {
			return m, nil, true
		}
		if msg.result.Error != nil {
			if !errors.Is(msg.result.Error, context.Canceled) {
				m.refreshErrors[msg.result.FeedID] = msg.result.Error
			}
		} else {
			delete(m.refreshErrors, msg.result.FeedID)
			if msg.result.FeedID > 0 {
				delete(m.refreshErrors, 0)
			}
		}
		cmd := tea.Batch(m.waitRefresh(), m.reload())
		if msg.result.Error == nil {
			cmd = tea.Batch(cmd, m.queueLatest(msg.result.FeedID))
			if msg.result.FeedID > 0 {
				engine, ctx, id := m.s.playback, m.s.ctx, msg.result.FeedID
				cmd = tea.Batch(cmd, work(ctx, "Updating chapter indicators…", func(ctx context.Context) tea.Msg {
					return chaptersSynced{err: engine.RefreshChapters(ctx, id)}
				}))
			}
		}
		return m, cmd, true
	default:
		return m, nil, false
	}
}

func (m libraryShell) updateInput(msg tea.Msg) (libraryShell, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if m.browse.searching {
			m.appendSearch(msg.Content)
			return m, nil, true
		}
		if m.adding {
			m.appendInput(msg.Content)
			return m, nil, true
		}
		updated, cmd := m.delegate(msg)
		return updated.(libraryShell), cmd, true
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	default:
		return m, nil, false
	}
}

// handleKey routes keyboard input through explicit precedence layers:
// 1. Application modals and text inputs (URL viewer, chapters, onboarding, confirmation, feed URL, search)
// 2. UI presentation modals (column filter, settings theme picker, help popup, message viewer)
// 3. Application actions for the focused context (transport, seeking, volume, queue/library mutations)
// 4. Default presentation navigation (vim motions, list selection, panel focus)
func (m libraryShell) handleKey(msg tea.KeyPressMsg) (libraryShell, tea.Cmd, bool) {
	// Layer 1: App modals & text inputs
	if updated, cmd, handled := m.tryModalKey(msg); handled {
		return updated, cmd, true
	}

	// Layer 2: Presentation modals (filter, settings, help, error viewer)
	if m.Model.Busy() {
		updated, cmd := m.delegate(msg)
		return updated.(libraryShell), cmd, true
	}

	// Layer 3: Application actions for focused context
	if updated, cmd, handled := m.handleActionKey(msg); handled {
		return updated, cmd, true
	}

	// Layer 4: Standard UI navigation
	updated, cmd := m.delegate(msg)
	return updated.(libraryShell), cmd, true
}

func (m libraryShell) tryModalKey(msg tea.KeyPressMsg) (libraryShell, tea.Cmd, bool) {
	if m.urlDialog != nil {
		switch msg.String() {
		case "esc", "I":
			m.urlDialog = nil
			m.Model = m.Model.WithDialog(nil)
		case "j", "down", "pgdown", "ctrl+d":
			m.urlDialog.Scroll++
		case "k", "up", "pgup", "ctrl+u":
			m.urlDialog.Scroll = max(0, m.urlDialog.Scroll-1)
		case "q", "ctrl+c":
			updated, cmd := m.delegate(msg)
			return updated.(libraryShell), cmd, true
		}
		return m, nil, true
	}
	if m.chapterList != nil {
		return m, m.chapterKey(msg.String()), true
	}
	if m.onboarding {
		return m, m.onboardingKey(msg), true
	}
	if m.confirming != nil {
		cmd := m.confirmationKey(msg)
		m.renderStatus()
		return m, cmd, true
	}
	if m.adding {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit, true
		}
		cmd := m.inputKey(msg)
		m.renderStatus()
		return m, cmd, true
	}
	if m.browse.searching {
		cmd := m.searchKey(msg)
		m.renderStatus()
		return m, cmd, true
	}
	return m, nil, false
}

func (m libraryShell) handleActionKey(msg tea.KeyPressMsg) (libraryShell, tea.Cmd, bool) {
	key := msg.String()
	focus := m.Model.Focus()
	var cmd tea.Cmd
	rebuild := false
	switch key {
	case "I":
		m.openCurrentURLs()
	case "C":
		cmd = m.openChapters()
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
			return m, nil, false
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
			return m, nil, false
		}
	case "=", "-":
		engine, ctx := m.s.playback, m.s.ctx
		delta := playback.VolumeStep
		if key == "-" {
			delta = -playback.VolumeStep
		}
		cmd = work(ctx, "Updating volume…", func(ctx context.Context) tea.Msg {
			state, err := engine.AdjustVolume(ctx, delta)
			return playerEdited{err: err, state: &state, volume: true}
		})
	case "space":
		engine, ctx := m.s.playback, m.s.ctx
		cmd = work(ctx, "Updating playback…", func(ctx context.Context) tea.Msg { return playerEdited{err: engine.Toggle(ctx, 0)} })
	case "i":
		m.openDetails()
	case "a", "A":
		cmd = m.enqueue(msg.String() == "A")
	case "J", "K":
		// Queue order is presentation-only; manual moves are no longer supported.
	case "m":
		if focus == ui.Queue {
			cmd = m.openQueueActions()
		}
	case "d", "delete":
		if focus == ui.Favorites {
			cmd = m.removeFavorite()
		} else if focus == ui.Queue {
			cmd = m.removeQueueItem()
		}
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
			if m.browse.retryPreview || m.browse.directURL {
				cmd = m.previewShow()
				m.renderLibrary()
				break
			}
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
			return m, nil, false
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
		} else if m.Model.Focus() == ui.Queue || m.Model.Focus() == ui.Episodes || m.Model.Focus() == ui.Details || m.Model.Focus() == ui.Favorites {
			cmd = m.playSelected()
		}
	default:
		return m, nil, false
	}
	if rebuild {
		m.renderLibrary()
		cmd = tea.Batch(cmd, m.syncLibrarySelection())
	} else {
		m.renderStatus()
	}
	return m, cmd, true
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
		if before.ID != after.ID {
			m.saveDiscoverViewSelection()
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
		m.notice = "Add feed canceled."
		m.Model = m.Model.ClearFeedback("Add feed")
	case "enter":
		if _, err := httpurl.Parse(strings.TrimSpace(m.input)); err != nil {
			m.report("Add feed", err, "")
			return nil
		}
		m.Model = m.Model.ClearFeedback("Add feed")
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
		data.EpisodeEmpty = "No saved episodes · f find a podcast"
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
	for _, fav := range m.snapshot.Favorites {
		item := episodeItem(fav, false)
		data.Favorites = append(data.Favorites, item)
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
	m.Model = m.Model.WithBackgroundFeedback(m.backgroundFeedback())
	if m.adding {
		prompt = "URL: " + m.input
	}
	m.Model = m.Model.WithStatus(notice, prompt)
}

func (m *libraryShell) renderOnboarding() {
	m.Model = m.Model.WithDialog(&ui.Dialog{
		Title:         "Welcome to podmutt",
		SectionLabels: []string{"Quick Start:"},
		Body: "A keyboard-driven podcast client for Linux and macOS.\n\n" +
			"Quick Start:\n" +
			"  • f         Find podcasts by name or paste an RSS/web URL\n" +
			"  • b         Toggle between Library and Discover (Top 100)\n" +
			"  • Enter     Play selected episode (or preview show)\n" +
			"  • Space     Pause / Resume playback from any pane\n" +
			"  • , / .     Seek backward / forward 15 seconds\n" +
			"  • a / d / u / m Enqueue episode / remove / undo removal / actions (favorite)\n" +
			"  • Tab / 1-6 Switch panels (Podcasts, Episodes, Queue, Details, Favorites, Player)\n" +
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
