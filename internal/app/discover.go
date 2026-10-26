package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/activity"
	"podmutt/internal/discovery"
	"podmutt/internal/feed"
	"podmutt/internal/feedback"
	"podmutt/internal/httpurl"
	"podmutt/internal/library"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

const discoveryActions = "f Search podcasts\nU RSS/website preview\nt Apple Top 100"

type showDirectory interface {
	Top(context.Context) ([]discovery.Show, error)
	discovery.Searcher
}

type freshShowDirectory interface {
	SearchFresh(context.Context, string) ([]discovery.Show, error)
	TopFresh(context.Context) ([]discovery.Show, error)
}
type browseState struct {
	indexDirectory                              discovery.Searcher
	usingIndex, searchIndex                     bool
	country, loadedKey, selectedFeedURL         string
	savedAt                                     time.Time
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
	input, errorText, inputError                string
	retryPreview                                bool
}
type discoverStartup struct{}
type directoryLoaded struct {
	generation                     uint64
	local                          bool
	provider, country, query, mode string
	shows                          []discovery.Show
	episodes                       []store.Episode
	err                            error
}
type previewLoaded struct {
	generation  uint64
	key, source string
	preview     feed.Resolved
	err         error
}
type showRatingLoaded struct {
	feedURL string
	rating  float64
	count   int
}

func (m *libraryShell) cancelPreview() {
	if m.browse.previewCancel != nil {
		m.browse.previewCancel()
		m.browse.previewCancel = nil
	}
	m.browse.previewGeneration++
	m.browse.preview = nil
	m.browse.retryPreview = false
	m.browse.previewKey = ""
	if m.detailTarget != nil && m.detailTarget.ID == 0 {
		m.detailTarget = nil
	}
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
func (m *libraryShell) clearDiscoveryFeedback() {
	for _, scope := range []string{"Apple Podcasts search", "Apple Podcasts chart", "Podcast Index search", "Feed preview", "Library search"} {
		m.Model = m.Model.ClearFeedback(scope)
	}
}
func (m *libraryShell) switchBrowse() tea.Cmd {
	m.clearDiscoveryFeedback()
	if m.browse.active {
		m.saveDiscoverViewSelection()
		m.discardIndexResults()
	}
	m.cancelDirectory()
	m.cancelPreview()
	m.browse.active = !m.browse.active
	m.browse.localResults = false
	m.Model = m.Model.ClearFilter(ui.Podcasts).ClearFilter(ui.Episodes).Reset(ui.Podcasts).Reset(ui.Episodes).WithFocus(ui.Podcasts)
	m.notice = "My library"
	var cmd tea.Cmd
	if m.browse.active {
		cmd = m.restoreDiscoverView()
	} else {
		m.wantLibraryID = m.libraryID
	}
	m.renderLibrary()
	return tea.Batch(cmd, m.syncLibrarySelection())
}

func (m *libraryShell) restoreDiscoverView() tea.Cmd {
	if m.s.store != nil {
		view, err := m.s.store.LoadDiscoverView(m.s.ctx)
		if err == nil {
			m.browse.usingIndex = view.Provider == "podcast_index"
			m.browse.country, m.browse.query, m.browse.selectedFeedURL = view.Country, view.Query, view.SelectedFeedURL
			if m.browse.country == "" {
				m.browse.country = "us"
			}
			m.browse.loadedKey = discoverKey(view.Provider, view.Country, view.Query, view.Mode)
			m.browse.shows, m.browse.loaded, m.browse.savedAt = nil, false, time.Time{}
			if view.Provider == "apple" {
				snapshot, err := m.s.store.LoadDiscoverSnapshot(m.s.ctx, view.Provider, view.Country, view.Query, view.Mode)
				if err == nil {
					var shows []discovery.Show
					if json.Unmarshal(snapshot.Results, &shows) == nil {
						m.browse.shows, m.browse.loaded, m.browse.savedAt = shows, true, snapshot.FetchedAt
					}
				}
			}
		}
	}
	m.browse.active = true
	m.renderLibrary()
	if m.browse.loaded {
		m.notice = fmt.Sprintf("Saved Apple Podcasts · %d results · updating…", len(m.browse.shows))
		if !m.browse.savedAt.IsZero() {
			m.notice += " · " + m.browse.savedAt.Local().Format("Jan 2 15:04")
		}
	}
	return m.searchDirectory(m.browse.query)
}

func discoverKey(provider, country, query, mode string) string {
	return strings.Join([]string{provider, strings.ToLower(country), strings.Join(strings.Fields(query), " "), mode}, "\x00")
}

func (m *libraryShell) saveDiscoverViewSelection() {
	if m.s.store == nil || !m.browse.active {
		return
	}
	selected := m.browse.selectedFeedURL
	if show, ok := m.selectedShow(); ok {
		selected = show.FeedURL
	}
	provider, mode := "apple", "chart"
	if m.browse.usingIndex {
		provider = "podcast_index"
	}
	if m.browse.query != "" {
		mode = "search"
	}
	if provider == "podcast_index" {
		selected = ""
	}
	_ = m.s.store.SaveDiscoverView(m.s.ctx, store.DiscoverView{Provider: provider, Country: m.browse.country, Query: m.browse.query, Mode: mode, SelectedFeedURL: selected})
}
func (m libraryShell) directoryName() string {
	if m.browse.usingIndex {
		return "Podcast Index"
	}
	return "Apple Podcasts"
}
func (m *libraryShell) searchDirectory(query string) tea.Cmd {
	m.clearDiscoveryFeedback()
	if query == "" {
		m.browse.usingIndex = false
	}
	m.cancelDirectory()
	m.browse.directURL = false
	m.browse.errorText = ""
	m.browse.active, m.browse.localResults, m.browse.query = true, false, strings.Join(strings.Fields(query), " ")
	query = m.browse.query
	providerName, mode := "apple", "chart"
	if m.browse.usingIndex {
		providerName = "podcast_index"
	}
	if query != "" {
		mode = "search"
	}
	country := strings.ToLower(m.browse.country)
	if country == "" {
		country = "us"
		m.browse.country = country
	}
	key := discoverKey(providerName, country, query, mode)
	if m.browse.loadedKey != "" && m.browse.loadedKey != key {
		m.cancelPreview()
	}
	selected := m.browse.selectedFeedURL
	if selected == "" {
		if show, ok := m.selectedShow(); ok {
			selected = show.FeedURL
		}
	}
	if key != m.browse.loadedKey {
		selected = ""
		m.browse.shows, m.browse.loaded, m.browse.savedAt = nil, false, time.Time{}
		if providerName == "apple" && m.s.store != nil {
			if snapshot, err := m.s.store.LoadDiscoverSnapshot(m.s.ctx, "apple", country, query, mode); err == nil {
				var cached []discovery.Show
				if json.Unmarshal(snapshot.Results, &cached) == nil {
					m.browse.shows, m.browse.loaded, m.browse.savedAt = cached, true, snapshot.FetchedAt
					if len(cached) > 0 {
						selected = cached[0].FeedURL
					}
				}
			}
		}
		m.browse.loadedKey = key
	}
	m.browse.selectedFeedURL = selected
	m.browse.loading = true
	if m.browse.loaded {
		m.notice = fmt.Sprintf("Saved %s · %d results · updating…", m.directoryName(), len(m.browse.shows))
	} else if query == "" {
		m.notice = "Loading " + m.directoryName() + "…"
	} else {
		m.notice = "Searching " + m.directoryName() + "…"
	}
	if providerName == "apple" && m.s.store != nil {
		_ = m.s.store.SaveDiscoverView(m.s.ctx, store.DiscoverView{Provider: providerName, Country: country, Query: query, Mode: mode, SelectedFeedURL: selected})
	} else if m.s.store != nil {
		_ = m.s.store.SaveDiscoverView(m.s.ctx, store.DiscoverView{Provider: providerName, Country: country, Query: query, Mode: mode})
	}
	ctx, cancel := context.WithCancel(m.s.ctx)
	m.browse.cancel = cancel
	generation, directory := m.browse.generation, m.browse.directory
	index, provider := m.browse.usingIndex, m.browse.indexDirectory
	m.renderLibrary()
	if selected != "" {
		m.Model = m.Model.Select(ui.Podcasts, selected)
		m.renderDetails()
	}
	label := "Searching " + m.directoryName() + "…"
	if query == "" {
		label = "Updating " + m.directoryName() + " charts…"
	}
	return work(ctx, label, func(ctx context.Context) tea.Msg {
		var shows []discovery.Show
		var err error
		if index {
			if provider == nil {
				err = discovery.ErrIndexCredentials
			} else {
				shows, err = provider.Search(ctx, query)
			}
		} else {
			if setter, ok := directory.(interface{ SetCountry(string) }); ok {
				setter.SetCountry(country)
			}
			if fresh, ok := directory.(freshShowDirectory); ok {
				if query == "" {
					shows, err = fresh.TopFresh(ctx)
				} else {
					shows, err = fresh.SearchFresh(ctx, query)
				}
			} else if query == "" {
				shows, err = directory.Top(ctx)
			} else {
				shows, err = directory.Search(ctx, query)
			}
		}
		return directoryLoaded{generation: generation, provider: providerName, country: country, query: query, mode: mode, shows: shows, err: err}
	})
}
func (m *libraryShell) searchLibrary(query string) tea.Cmd {
	m.clearDiscoveryFeedback()
	m.discardIndexResults()
	m.cancelDirectory()
	m.cancelPreview()
	m.browse.active = false
	m.browse.localQuery = query
	m.browse.localResults = query != ""
	m.browse.hits = nil
	if query == "" {
		m.notice = "My library"
		m.wantLibraryID = m.libraryID
		m.renderLibrary()
		return m.syncLibrarySelection()
	}
	m.notice = "Searching cached subscriptions…"
	m.browse.loading = true
	ctx, cancel := context.WithCancel(m.s.ctx)
	m.browse.cancel = cancel
	generation, service := m.browse.generation, m.service
	m.renderLibrary()
	return work(ctx, "Searching saved library…", func(ctx context.Context) tea.Msg {
		episodes, err := service.Search(ctx, query)
		return directoryLoaded{generation: generation, local: true, episodes: episodes, err: err}
	})
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
	if errors.Is(msg.err, context.Canceled) {
		m.notice, m.browse.errorText = "", ""
		m.renderLibrary()
		return
	}
	scope, retry := m.directoryName()+" search", "Press R to retry, f for another search, or b for your saved Library."
	if msg.local {
		scope, retry = "Library search", "Press f to search your saved Library again."
	} else if m.browse.query == "" {
		scope = m.directoryName() + " chart"
	}
	if msg.err == nil {
		m.Model = m.Model.ClearFeedback(scope)
		m.browse.errorText = ""
	}
	if msg.err != nil {
		if !msg.local && m.browse.loaded {
			message := feedback.Retry(feedback.For(scope, msg.err), "Showing saved results. Press R to retry.")
			m.Model = m.Model.WithFeedback(message)
			m.notice = fmt.Sprintf("Saved %s · update failed · showing saved results · R retry", m.directoryName())
		} else {
			m.report(scope, msg.err, retry)
			m.browse.errorText = m.notice
		}
	} else if msg.local {
		m.browse.hits = msg.episodes
		m.notice = fmt.Sprintf("Library search · %d saved episodes", len(msg.episodes))
		if len(msg.episodes) == 100 {
			m.notice += " · showing up to 100 matches"
		}
	} else {
		previous := m.browse.selectedFeedURL
		if item, ok := m.Model.Selected(ui.Podcasts); ok && item.ID != "" {
			previous = item.ID
		}
		shows := msg.shows
		if shows == nil {
			shows = []discovery.Show{}
		}
		m.browse.shows = append([]discovery.Show(nil), shows...)
		m.browse.loaded = true
		m.browse.savedAt = time.Now().UTC()
		m.browse.loadedKey = discoverKey(msg.provider, msg.country, msg.query, msg.mode)
		if m.browse.selectedFeedURL != "" {
			previous = m.browse.selectedFeedURL
		}
		found := false
		for _, show := range msg.shows {
			if show.FeedURL == previous {
				found = true
				break
			}
		}
		if found {
			m.browse.selectedFeedURL = previous
		} else {
			if m.browse.preview != nil {
				m.cancelPreview()
			}
			m.browse.selectedFeedURL = ""
			if len(msg.shows) > 0 {
				m.browse.selectedFeedURL = msg.shows[0].FeedURL
			}
		}
		if msg.provider == "apple" && m.s.store != nil {
			results, _ := json.Marshal(shows)
			_ = m.s.store.SaveDiscoverSnapshot(m.s.ctx, store.DiscoverSnapshot{Provider: "apple", Country: msg.country, Query: msg.query, Mode: msg.mode, Results: results, FetchedAt: m.browse.savedAt})
			_ = m.s.store.SaveDiscoverView(m.s.ctx, store.DiscoverView{Provider: "apple", Country: msg.country, Query: msg.query, Mode: msg.mode, SelectedFeedURL: m.browse.selectedFeedURL})
		}
		source := m.directoryName()
		if !m.browse.usingIndex && msg.country != "" && strings.ToUpper(msg.country) != "US" {
			source += " (" + strings.ToUpper(msg.country) + ")"
		}
		m.notice = fmt.Sprintf("%s · %d shows · Enter preview · u subscribe", source, len(msg.shows))
		if m.s.store != nil && len(msg.shows) > 0 && msg.provider == "apple" {
			if sourceID, err := m.s.store.SourceID(m.s.ctx, "apple"); err == nil && sourceID > 0 {
				var toStore []store.DiscoveredShow
				for _, show := range msg.shows {
					toStore = append(toStore, store.DiscoveredShow{
						Title:            show.Title,
						FeedURL:          show.FeedURL,
						ArtworkURL:       show.SourceURL,
						Country:          msg.country,
						SourceID:         sourceID,
						SourceExternalID: show.ExternalID,
					})
				}
				_ = m.s.store.UpsertDiscoveredShows(m.s.ctx, msg.country, sourceID, toStore)
			}
		}
	}
	m.renderLibrary()
	if !msg.local && msg.err == nil && m.browse.selectedFeedURL != "" {
		m.Model = m.Model.Select(ui.Podcasts, m.browse.selectedFeedURL)
		m.renderDetails()
	}
	if msg.local && msg.err == nil && len(m.Model.Visible(ui.Podcasts)) == 0 && len(m.Model.Visible(ui.Episodes)) > 0 {
		m.Model = m.Model.WithFocus(ui.Episodes)
		m.renderDetails()
	}
	if !msg.local && msg.err == nil {
		m.ensureShowRating()
	}
}
func (m *libraryShell) openSearch() {
	m.browse.searching = true
	m.browse.urlInput = false
	m.browse.searchApple = m.browse.active
	m.browse.searchIndex = m.browse.usingIndex
	m.browse.inputError = ""
	m.browse.input = ""
	m.renderSearch()
}
func (m *libraryShell) openURL() {
	m.browse.searching = true
	m.browse.urlInput = true
	m.browse.inputError = ""
	m.browse.input = ""
	m.renderSearch()
}
func (m *libraryShell) renderSearch() {
	if m.browse.urlInput {
		input := m.browse.input
		body := "HTTP(S) podcast feed or website URL.\nRead-only preview. Enter plays; u subscribes."
		if m.browse.inputError != "" {
			body = m.browse.inputError
		}
		m.Model = m.Model.WithDialog(&ui.Dialog{
			Title:      "RSS or Website Preview",
			Body:       body,
			InputLabel: "URL",
			InputValue: input,
			Hint:       "Esc cancel · Enter preview · Ctrl+U clear",
		})
		return
	}
	title := "Search: My Subscriptions"
	scope := "Offline search: show names and cached episode titles/descriptions"
	if m.browse.searchApple {
		title = "Search: Apple Podcasts"
		scope = "Online shows directory (no remote episode search)"
		if m.browse.searchIndex {
			title = "Search: Podcast Index"
			scope = "Online shows directory (Podcast Index credentials)"
		}
	}
	body := scope + "\nOr paste a feed/website URL for a read-only preview."
	if m.browse.inputError != "" {
		body = m.browse.inputError
	}
	m.Model = m.Model.WithDialog(&ui.Dialog{
		Title:      title,
		Body:       body,
		InputLabel: "Query or URL",
		InputValue: m.browse.input,
		Hint:       "Esc cancel · Enter submit · Tab source · Ctrl+U clear",
	})
}
func (m *libraryShell) appendSearch(text string) {
	ti := ui.TextInput{Value: m.browse.input, Limit: m.searchLimit(m.browse.input + text)}
	ti.Append(text)
	m.browse.input = ti.Value
	m.renderSearch()
}

// Recognize URL intent before provider dispatch, including unsupported schemes.
// Do not send pasted URLs (possibly containing credentials) to a directory.
func (m libraryShell) searchingURL(input string) bool {
	input = strings.ToLower(strings.TrimSpace(input))
	return m.browse.urlInput || strings.Contains(input, "://") ||
		strings.HasPrefix(input, "http:") || strings.HasPrefix(input, "https:")
}

func (m libraryShell) searchLimit(input string) int {
	if m.searchingURL(input) {
		return 4096
	}
	return 128
}
func (m *libraryShell) searchKey(key tea.KeyPressMsg) tea.Cmd {
	switch key.String() {
	case "ctrl+c":
		return tea.Quit
	case "esc":
		m.notice = "Search canceled."
		if m.browse.urlInput {
			m.notice = "URL preview canceled."
		}
		m.browse.input, m.browse.inputError = "", ""
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
	case "ctrl+u", "backspace":
		ti := ui.TextInput{Value: m.browse.input, Limit: m.searchLimit(m.browse.input)}
		ti.EditKey(key)
		m.browse.input = ti.Value
		m.renderSearch()
	case "enter":
		if m.searchingURL(m.browse.input) {
			address, err := httpurl.Parse(strings.TrimSpace(m.browse.input))
			if err != nil {
				m.browse.inputError = feedback.For("Feed preview", err).Summary
				m.notice = m.browse.inputError
				m.renderSearch()
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
			m.browse.inputError = m.notice
			m.renderSearch()
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
	m.browse.retryPreview = true
	m.Model = m.Model.ClearFeedback("Feed preview")
	m.browse.errorText = ""
	m.notice = "Loading feed preview…"
	ctx, cancel := context.WithCancel(m.s.ctx)
	m.browse.previewCancel = cancel
	generation, service := m.browse.previewGeneration, m.service
	previewCmd := work(ctx, "Loading feed preview…", func(ctx context.Context) tea.Msg {
		preview, err := service.Preview(ctx, show.FeedURL)
		return previewLoaded{generation, show.FeedURL, show.SourceURL, preview, err}
	})
	return tea.Batch(previewCmd, m.ensureShowRating())
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
	if errors.Is(msg.err, context.Canceled) {
		m.notice, m.browse.errorText = "", ""
		m.renderLibrary()
		return
	}
	if msg.err != nil {
		m.report("Feed preview", msg.err, "Press R to retry this feed preview, or U to enter a different URL.")
		m.browse.errorText = m.notice
	} else {
		m.Model = m.Model.ClearFeedback("Feed preview")
		m.browse.errorText = ""
		m.browse.preview = &msg.preview
		m.browse.previewKey = msg.key
		m.browse.source = msg.source
		m.Model = m.Model.Reset(ui.Episodes)
		m.notice = "Tab/2 episodes · Enter play · a queue · u subscribe · browsing fetched no audio"
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
	return work(ctx, "Subscribing to podcast…", func(ctx context.Context) tea.Msg {
		id, err := service.SubscribePreview(ctx, preview)
		return subscriptionEdited(ctx, engine, id, true, err)
	})
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
	topTag := "US Top 100"
	if m.browse.country != "" && strings.ToUpper(m.browse.country) != "US" {
		topTag = strings.ToUpper(m.browse.country) + " Top 100"
	}
	label := "Discover · Apple Podcasts · " + topTag
	if m.browse.query != "" {
		label = "Discover · Apple Podcasts search: " + m.browse.query
		if m.browse.usingIndex {
			label = "Discover · Podcast Index search: " + m.browse.query
		}
	}
	if m.browse.directURL {
		label = "RSS/website · read-only preview"
	}
	data := ui.Data{PodcastLabel: label, EpisodeLabel: "Preview · " + m.order.String() + " · Enter plays without subscribing"}
	data.PodcastEmpty = "No matching shows · f search"
	if m.browse.loading {
		data.PodcastEmpty = "Loading " + m.directoryName() + "…"
	} else if m.browse.errorText != "" {
		data.PodcastEmpty = "Request failed · 4 details"
	}
	for _, show := range m.browse.shows {
		marker := ""
		for _, source := range m.snapshot.Feeds {
			if source.Subscribed && source.URL == show.FeedURL {
				marker = "[+] "
				break
			}
		}
		title := library.PlainText(show.Title)
		data.Podcasts = append(data.Podcasts, ui.Item{ID: show.FeedURL, Title: marker + title, Channel: title})
	}
	if m.browse.preview != nil {
		previewShowTitle := library.PlainText(m.browse.preview.Fetched.Document.Title)
		// Reuse library sorting on a position-tagged copy; row IDs still address
		// the original preview document, never its transient display positions.
		positions := append([]store.Episode(nil), m.browse.preview.Fetched.Document.Episodes...)
		for i := range positions {
			positions[i].ID = int64(i)
		}
		for _, position := range library.Episodes(library.Snapshot{Show: positions}, library.ShowView, m.order) {
			i := int(position.ID)
			episode := m.browse.preview.Fetched.Document.Episodes[i]
			item := previewEpisodeItem(i, episode, previewShowTitle)
			saved := m.detailHistory(episode)
			item.Current = saved.ID > 0 && m.state.Current != nil && m.state.Current.ID == saved.ID
			item.Queued = item.Current && currentInQueue(m.state)
			for _, pending := range m.state.Queue {
				if saved.ID > 0 && pending.ID == saved.ID {
					item.Queued = true
				}
			}
			data.Episodes = append(data.Episodes, item)
		}
	}
	for _, fav := range m.snapshot.Favorites {
		item := episodeItem(fav, false)
		data.Favorites = append(data.Favorites, item)
	}
	m.Model = m.Model.WithLibrary(data)
	m.renderDetails()
	m.renderStatus()
}
func (m *libraryShell) renderPreviewDetails() {
	country := strings.ToUpper(m.browse.country)
	if country == "" {
		country = "US"
	}
	details := "Apple Podcasts · " + country + " Top 100\nEnter a show to inspect its feed before subscribing.\nf searches Apple shows; b returns to Library.\nSource: https://podcasts.apple.com/" + strings.ToLower(country) + "/charts"
	if m.browse.query != "" {
		details = m.directoryName() + " search: " + m.browse.query + "\nOnline shows, not remote episode text."
		if m.browse.usingIndex {
			details += "\nSource: https://podcastindex.org"
		}
	}
	if show, ok := m.selectedShow(); ok && show.Rating > 0 {
		details += fmt.Sprintf("\nRating: ★ %.1f/5 (%d reviews)", show.Rating, show.ReviewCount)
	}
	if m.browse.directURL {
		details = "RSS/website · Enter preview · u explicitly subscribes"
	}
	if m.browse.loading && !m.browse.loaded {
		details = "Loading " + m.directoryName() + "…\nQuery: " + m.browse.query + "\nEsc cancel · b cached subscriptions (offline)"
	} else if m.browse.errorText != "" {
		details = m.browse.errorText
	} else if len(m.browse.shows) == 0 && !m.browse.directURL {
		details += "\nNo matching shows. f Search podcasts · U RSS/website · t chart"
	}
	if m.browse.previewCancel != nil {
		details = "Loading RSS/feed preview…\nEsc cancels; no subscription or autoplay."
	}
	if m.browse.preview != nil {
		details = library.PlainText(m.browse.preview.Fetched.Document.Title) + "\nEnter play · i details · a enqueue · u subscribe"
		if show, ok := m.selectedShow(); ok && show.Rating > 0 {
			details += fmt.Sprintf("\nCustomer rating: ★ %.1f/5 (%d reviews)", show.Rating, show.ReviewCount)
		}
		if m.browse.source != "" {
			details += "\nDirectory source: " + feedback.SourceURL(m.browse.source)
		} else if m.browse.directURL {
			details += "\nFeed: " + feedback.SourceURL(m.browse.preview.URL)
		}
	}
	if m.browse.loading && m.browse.loaded {
		details += "\nSaved results · updating in background"
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
	m.libraryID = fmt.Sprint(id)
	m.wantLibraryID = m.libraryID
}

func (m *libraryShell) ensureShowRating() tea.Cmd {
	show, ok := m.selectedShow()
	if !ok || show.Rating > 0 || show.ExternalID == "" {
		return nil
	}
	if m.s.store != nil {
		shows, err := m.s.store.DiscoveredShows(m.s.ctx, m.browse.country, show.Title, 5)
		if err == nil {
			for _, s := range shows {
				if s.FeedURL == show.FeedURL && s.Rating > 0 {
					for i := range m.browse.shows {
						if m.browse.shows[i].FeedURL == show.FeedURL {
							m.browse.shows[i].Rating = s.Rating
							m.browse.shows[i].ReviewCount = s.ReviewCount
							m.renderDetails()
							return nil
						}
					}
				}
			}
		}
	}
	apple, ok := m.browse.directory.(*discovery.Apple)
	if !ok {
		return nil
	}
	ctx := m.s.ctx
	country := m.browse.country
	id := show.ExternalID
	feedURL := show.FeedURL
	return trackedWork(ctx, "Loading podcast ratings…", activity.Background, func(ctx context.Context) tea.Msg {
		r, c, _ := apple.Ratings(ctx, country, id)
		return showRatingLoaded{feedURL: feedURL, rating: r, count: c}
	})
}

func (m libraryShell) updateDiscovery(msg tea.Msg) (libraryShell, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case discoverStartup:
		if m.browse.active && !m.browse.loaded && !m.browse.loading {
			return m, m.restoreDiscoverView(), true
		}
		return m, nil, true
	case directoryLoaded:
		m.applyDirectory(msg)
		return m, nil, true
	case previewLoaded:
		m.applyPreview(msg)
		return m, nil, true
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
		return m, nil, true
	default:
		return m, nil, false
	}
}
