package app

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/discovery"
	"podmutt/internal/feed"
	"podmutt/internal/httpurl"
	"podmutt/internal/library"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

const discoveryActions = "f Search podcasts\nU RSS/website preview\nt Apple US top 25"

type showDirectory interface {
	Top(context.Context) ([]discovery.Show, error)
	discovery.Searcher
}
type browseState struct {
	indexDirectory                              discovery.Searcher
	usingIndex, searchIndex                     bool
	directory                                   showDirectory
	active, loaded, loading, localResults       bool
	shows                                       []discovery.Show
	hits                                        []store.Episode
	query, localQuery                           string
	generation                                  uint64
	cancel                                      context.CancelFunc
	preview                                     *feed.Resolved
	previewKey, source                          string
	previewGeneration                           uint64
	previewCancel                               context.CancelFunc
	searching, searchApple, urlInput, directURL bool
	input, errorText                            string
}
type discoverStartup struct{}
type directoryLoaded struct {
	generation uint64
	local      bool
	shows      []discovery.Show
	episodes   []store.Episode
	err        error
}
type previewLoaded struct {
	generation  uint64
	key, source string
	preview     feed.Resolved
	err         error
}

func (m *libraryShell) cancelPreview() {
	if m.browse.previewCancel != nil {
		m.browse.previewCancel()
		m.browse.previewCancel = nil
	}
	m.browse.previewGeneration++
	m.browse.preview = nil
	m.browse.previewKey = ""
	m.detailTarget = nil
}
func (m *libraryShell) cancelDirectory() {
	if m.browse.cancel != nil {
		m.browse.cancel()
		m.browse.cancel = nil
	}
	m.browse.generation++
	m.browse.loading = false
}
func (m *libraryShell) discardIndexResults() {
	if m.browse.usingIndex {
		m.browse.shows = nil
		m.browse.loaded = false
	}
}
func (m *libraryShell) switchBrowse() tea.Cmd {
	if m.browse.active {
		m.discardIndexResults()
	}
	m.cancelDirectory()
	m.cancelPreview()
	m.browse.active = !m.browse.active
	m.browse.localResults = false
	m.Model = m.Model.ClearFilter(ui.Podcasts).ClearFilter(ui.Episodes).Reset(ui.Podcasts).Reset(ui.Episodes).WithFocus(ui.Podcasts)
	m.notice = "My subscriptions"
	var cmd tea.Cmd
	if m.browse.active {
		m.notice = "Apple Podcasts · Enter preview · u subscribe"
		if !m.browse.loaded {
			cmd = m.searchDirectory(m.browse.query)
		}
	} else {
		m.wantFeedID = m.feedID
	}
	m.renderLibrary()
	return cmd
}
func (m libraryShell) directoryName() string {
	if m.browse.usingIndex {
		return "Podcast Index"
	}
	return "Apple Podcasts"
}
func (m *libraryShell) searchDirectory(query string) tea.Cmd {
	if query == "" {
		m.browse.usingIndex = false
	}
	m.cancelDirectory()
	m.cancelPreview()
	m.browse.shows = nil
	m.browse.directURL = false
	m.browse.errorText = ""
	m.browse.active = true
	m.browse.localResults = false
	m.browse.query = query
	m.browse.loading = true
	m.notice = "Loading " + m.directoryName() + "…"
	ctx, cancel := context.WithCancel(m.s.ctx)
	m.browse.cancel = cancel
	generation, directory := m.browse.generation, m.browse.directory
	index, provider := m.browse.usingIndex, m.browse.indexDirectory
	m.renderLibrary()
	return func() tea.Msg {
		var shows []discovery.Show
		var err error
		if index {
			if provider == nil {
				err = discovery.ErrIndexCredentials
			} else {
				shows, err = provider.Search(ctx, query)
			}
		} else if query == "" {
			shows, err = directory.Top(ctx)
		} else {
			shows, err = directory.Search(ctx, query)
		}
		return directoryLoaded{generation: generation, shows: shows, err: err}
	}
}
func (m *libraryShell) searchLibrary(query string) tea.Cmd {
	m.discardIndexResults()
	m.cancelDirectory()
	m.cancelPreview()
	m.browse.active = false
	m.browse.localQuery = query
	m.browse.localResults = query != ""
	m.browse.hits = nil
	if query == "" {
		m.notice = "My subscriptions"
		m.renderLibrary()
		return nil
	}
	m.notice = "Searching cached subscriptions…"
	m.browse.loading = true
	ctx, cancel := context.WithCancel(m.s.ctx)
	m.browse.cancel = cancel
	generation, service := m.browse.generation, m.service
	m.renderLibrary()
	return func() tea.Msg {
		episodes, err := service.Search(ctx, query)
		return directoryLoaded{generation: generation, local: true, episodes: episodes, err: err}
	}
}
func (m *libraryShell) applyDirectory(msg directoryLoaded) {
	if msg.generation != m.browse.generation || msg.local == m.browse.active {
		return
	}
	if m.browse.cancel != nil {
		m.browse.cancel()
		m.browse.cancel = nil
	}
	m.browse.loading = false
	if msg.err != nil {
		m.notice = msg.err.Error()
		m.browse.errorText = m.notice
	} else if msg.local {
		m.browse.hits = msg.episodes
		m.notice = fmt.Sprintf("Library search · %d cached episodes (max 100)", len(msg.episodes))
	} else {
		m.browse.shows = msg.shows
		m.browse.loaded = true
		m.notice = fmt.Sprintf("%s · %d shows · Enter preview · u subscribe", m.directoryName(), len(msg.shows))
	}
	m.renderLibrary()
	if msg.local && msg.err == nil && len(m.Model.Visible(ui.Podcasts)) == 0 && len(m.Model.Visible(ui.Episodes)) > 0 {
		m.Model = m.Model.WithFocus(ui.Episodes)
		m.renderDetails()
	}
}
func (m *libraryShell) openSearch() {
	m.browse.searching = true
	m.browse.urlInput = false
	m.browse.searchApple = m.browse.active
	m.browse.searchIndex = m.browse.usingIndex
	m.browse.input = ""
	m.renderSearch()
}
func (m *libraryShell) openURL() {
	m.browse.searching = true
	m.browse.urlInput = true
	m.browse.input = ""
	m.renderSearch()
}
func (m *libraryShell) renderSearch() {
	if m.browse.urlInput {
		input := m.browse.input
		if runes := []rune(input); len(runes) > 96 {
			input = "…" + string(runes[len(runes)-96:])
		}
		m.Model = m.Model.WithDialog(&ui.Dialog{Title: "RSS/feed or website preview", Body: "HTTP(S) podcast feed or website URL:\n" + input + "\n\nRead-only preview. Space plays; u explicitly subscribes.\nN is the separate add-and-subscribe shortcut.", Hint: "Enter preview · Esc cancel · Ctrl+U clear"})
		return
	}
	scope := "My subscriptions (offline): show names + cached episode titles/descriptions"
	if m.browse.searchApple {
		scope = "Apple Podcasts: online shows (no remote episode search)"
		if m.browse.searchIndex {
			scope = "Podcast Index: online shows · your own API credentials required\nSource: https://podcastindex.org · see docs/podcast-index.md"
		}
	}
	m.Model = m.Model.WithDialog(&ui.Dialog{Title: "Search podcasts · beyond the top 25", Body: scope + "\n\nQuery: " + m.browse.input, Hint: "Enter search · Tab switch scope · Esc cancel · Ctrl+U clear"})
}
func (m *libraryShell) appendSearch(text string) {
	// Treat pasted control characters as whitespace, not commands or escapes.
	text = strings.Map(func(r rune) rune {
		if r < ' ' || r == 127 {
			return ' '
		}
		return r
	}, text)
	runes := []rune(m.browse.input + text)
	m.browse.input = string(runes[:min(len(runes), m.searchLimit())])
	m.renderSearch()
}
func (m libraryShell) searchLimit() int {
	if m.browse.urlInput {
		return 4096
	}
	return 128
}
func (m *libraryShell) searchKey(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "ctrl+c":
		return tea.Quit
	case "esc":
		m.browse.searching = false
		m.Model = m.Model.WithDialog(nil)
	case "tab":
		if !m.browse.urlInput {
			if !m.browse.searchApple {
				m.browse.searchApple = true
				m.browse.searchIndex = false
			} else if !m.browse.searchIndex {
				m.browse.searchIndex = true
			} else {
				m.browse.searchApple = false
				m.browse.searchIndex = false
			}
			m.renderSearch()
		}
	case "ctrl+u":
		m.browse.input = ""
		m.renderSearch()
	case "backspace":
		runes := []rune(m.browse.input)
		m.browse.input = string(runes[:max(0, len(runes)-1)])
		m.renderSearch()
	case "enter":
		if m.browse.urlInput {
			address, err := httpurl.Parse(strings.TrimSpace(m.browse.input))
			if err != nil {
				m.notice = err.Error()
				return nil
			}
			m.browse.searching = false
			m.Model = m.Model.WithDialog(nil)
			m.cancelDirectory()
			m.cancelPreview()
			m.browse.active = true
			m.browse.usingIndex = false
			m.browse.localResults = false
			m.browse.directURL = true
			m.browse.errorText = ""
			m.browse.query = ""
			m.browse.shows = []discovery.Show{{Title: "RSS/website preview", FeedURL: address.String()}}
			m.Model = m.Model.ClearFilter(ui.Podcasts).ClearFilter(ui.Episodes).Reset(ui.Podcasts).Reset(ui.Episodes).WithFocus(ui.Podcasts)
			m.renderLibrary()
			cmd := m.previewShow()
			m.renderLibrary()
			return cmd
		}
		query := strings.Join(strings.Fields(m.browse.input), " ")
		if m.browse.searchApple && m.browse.searchIndex && query == "" {
			m.notice = "Podcast Index requires a search term"
			return nil
		}
		m.browse.searching = false
		m.Model = m.Model.WithDialog(nil)
		m.Model = m.Model.ClearFilter(ui.Podcasts).ClearFilter(ui.Episodes).Reset(ui.Podcasts).Reset(ui.Episodes).WithFocus(ui.Podcasts)
		if m.browse.searchApple {
			m.browse.usingIndex = m.browse.searchIndex
			return m.searchDirectory(query)
		}
		return m.searchLibrary(query)
	default:
		m.appendSearch(key.Text)
	}
	return nil
}
func (m libraryShell) selectedShow() (discovery.Show, bool) {
	item, ok := m.Model.Selected(ui.Podcasts)
	if !ok {
		return discovery.Show{}, false
	}
	for _, show := range m.browse.shows {
		if show.FeedURL == item.ID {
			return show, true
		}
	}
	return discovery.Show{}, false
}
func (m *libraryShell) previewShow() tea.Cmd {
	show, ok := m.selectedShow()
	if !ok {
		m.notice = "Select a show first"
		return nil
	}
	m.cancelPreview()
	m.notice = "Loading feed preview…"
	ctx, cancel := context.WithCancel(m.s.ctx)
	m.browse.previewCancel = cancel
	generation, service := m.browse.previewGeneration, m.service
	return func() tea.Msg {
		preview, err := service.Preview(ctx, show.FeedURL)
		return previewLoaded{generation, show.FeedURL, show.SourceURL, preview, err}
	}
}
func (m *libraryShell) applyPreview(msg previewLoaded) {
	show, ok := m.selectedShow()
	if !m.browse.active || msg.generation != m.browse.previewGeneration || !ok || show.FeedURL != msg.key {
		return
	}
	if m.browse.previewCancel != nil {
		m.browse.previewCancel()
		m.browse.previewCancel = nil
	}
	if msg.err != nil {
		m.notice = msg.err.Error()
		m.browse.errorText = m.notice
	} else {
		m.browse.errorText = ""
		m.browse.preview = &msg.preview
		m.browse.previewKey = msg.key
		m.browse.source = msg.source
		m.Model = m.Model.Reset(ui.Episodes).WithFocus(ui.Episodes)
		m.notice = "Space play · a enqueue · u subscribe · browsing fetched no audio"
	}
	m.renderLibrary()
}
func (m *libraryShell) subscribePreview() tea.Cmd {
	if m.mutating {
		return nil
	}
	if m.browse.preview == nil {
		m.notice = "Enter a show to preview before subscribing"
		return nil
	}
	m.mutating = true
	m.notice = "Subscribing…"
	preview, service, ctx, engine := *m.browse.preview, m.service, m.s.ctx, m.s.playback
	return func() tea.Msg {
		id, err := service.SubscribePreview(ctx, preview)
		return subscriptionEdited(ctx, engine, id, true, err)
	}
}
func (m libraryShell) previewEpisode() (store.Episode, bool) {
	if m.browse.preview == nil {
		return store.Episode{}, false
	}
	item, ok := m.Model.Selected(ui.Episodes)
	if !ok {
		return store.Episode{}, false
	}
	for i, episode := range m.browse.preview.Fetched.Document.Episodes {
		if item.ID == fmt.Sprintf("preview:%d", i) {
			return episode, true
		}
	}
	return store.Episode{}, false
}
func (m *libraryShell) renderDiscovery() {
	label := "Discover · Apple Podcasts · US top 25"
	if m.browse.query != "" {
		label = "Discover · Apple search: " + m.browse.query
		if m.browse.usingIndex {
			label = "Discover · Podcast Index search: " + m.browse.query
		}
	}
	if m.browse.directURL {
		label = "RSS/website · read-only preview"
	}
	data := ui.Data{PodcastHint: discoveryActions, PodcastLabel: label, EpisodeLabel: "Preview · Space plays without subscribing", ContextHint: "Space play · a/A enqueue · u subscribe · f Search podcasts · U RSS/website · t chart · b My subscriptions · q quit"}
	data.PodcastEmpty = "No matching shows · f search"
	if m.browse.loading {
		data.PodcastEmpty = "Loading " + m.directoryName() + "…"
	} else if m.browse.errorText != "" {
		data.PodcastEmpty = "Search failed · 4 details"
	}
	for _, show := range m.browse.shows {
		marker := "    "
		for _, source := range m.snapshot.Feeds {
			if source.Subscribed && source.URL == show.FeedURL {
				marker = "[+] "
				break
			}
		}
		data.Podcasts = append(data.Podcasts, ui.Item{ID: show.FeedURL, Title: marker + library.PlainText(show.Title)})
	}
	if m.browse.preview != nil {
		for i, episode := range m.browse.preview.Fetched.Document.Episodes {
			data.Episodes = append(data.Episodes, ui.Item{ID: fmt.Sprintf("preview:%d", i), Title: library.PlainText(episode.Title)})
		}
	}
	m.Model = m.Model.WithLibrary(data)
	m.renderDetails()
	m.renderStatus()
}
func (m *libraryShell) renderPreviewDetails() {
	details := "Apple Podcasts · US top 25\nEnter a show to inspect its feed before subscribing.\nf searches Apple shows; b returns to My subscriptions.\nSource: https://podcasts.apple.com/us/charts"
	if m.browse.query != "" {
		details = m.directoryName() + " search: " + m.browse.query + "\nOnline shows, not remote episode text."
		if m.browse.usingIndex {
			details += "\nSource: https://podcastindex.org"
		}
	}
	if m.browse.directURL {
		details = "RSS/website · Enter preview · u explicitly subscribes"
	}
	if m.browse.loading {
		details = "Loading " + m.directoryName() + "…\nQuery: " + m.browse.query + "\nEsc cancel · b cached subscriptions (offline)"
	} else if m.browse.errorText != "" {
		details = "Discovery failed: " + m.browse.errorText + "\nf search again · t top 25 · b cached subscriptions"
	} else if len(m.browse.shows) == 0 && !m.browse.directURL {
		details += "\nNo matching shows. f Search podcasts · U RSS/website · t chart"
	}
	if m.browse.previewCancel != nil {
		details = "Loading RSS/feed preview…\nEsc cancels; no subscription or autoplay."
	}
	if m.browse.preview != nil {
		details = library.PlainText(m.browse.preview.Fetched.Document.Title) + "\nSpace play · a enqueue · u subscribe"
		if m.browse.source != "" {
			details += "\nDirectory source: " + m.browse.source
		} else if m.browse.directURL {
			details += "\nFeed: " + m.browse.preview.URL
		}
		episode, ok := m.previewEpisode()
		if m.Model.Focus() == ui.Details && m.detailTarget != nil {
			episode, ok = *m.detailTarget, true
		}
		if ok {
			details += "\n\n" + library.PlainText(episode.Title) + "\n" + library.PlainText(episode.Description)
		}
	}
	m.Model = m.Model.WithDetails(details)
}
func (m libraryShell) subscribedCount() int {
	n := 0
	for _, source := range m.snapshot.Feeds {
		if source.Subscribed {
			n++
		}
	}
	return n
}
func (m libraryShell) matchesShow(source store.Feed) bool {
	return !m.browse.localResults || strings.Contains(strings.ToLower(strings.Join(strings.Fields(library.PlainText(source.Title)), " ")), strings.ToLower(m.browse.localQuery))
}
func (m *libraryShell) selectLibraryShow() {
	id := m.selectedFeed()
	m.browse.localResults = false
	m.browse.localQuery = ""
	m.feedID = id
	m.wantFeedID = id
}
