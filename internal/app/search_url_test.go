package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/ui"
)

func TestSearchAcceptsURLInEverySourceWithoutSubscribing(t *testing.T) {
	server := urlPreviewServer(t)
	for _, source := range []string{"Library", "Apple", "Index"} {
		t.Run(source, func(t *testing.T) {
			librarySession(t, func(s *session) {
				m, p := playerLibrary(t, s)
				apple, index := &fixtureDirectory{}, &fixtureDirectory{}
				m.browse.directory, m.browse.indexDirectory = apple, index
				for _, size := range [][2]int{{140, 40}, {70, 20}, {30, 12}} {
					updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
					libraryKey(&m, 'f')
					m.browse.searchApple = source != "Library"
					m.browse.searchIndex = source == "Index"
					m.renderSearch()
					// Full RSS URLs longer than directory-query limits must survive paste.
					address := server.URL + "/rss?token=" + strings.Repeat("x", 200)
					updateLibrary(&m, tea.PasteMsg{Content: address})
					if m.browse.input != address {
						t.Fatal("URL truncated to query length", len(m.browse.input))
					}
					finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
					if m.browse.preview == nil || m.browse.preview.URL != address || m.Model.Focus() != ui.Podcasts || m.browse.searching {
						t.Fatal("search did not open feed preview", m.notice)
					}
					libraryKey(&m, 'f')
					updateLibrary(&m, tea.PasteMsg{Content: "  " + server.URL + "/site  "})
					finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
					if m.browse.preview == nil || m.browse.preview.URL != server.URL+"/rss" {
						t.Fatal("search did not resolve website", m.notice)
					}
				}
				feeds, err := s.store.Feeds(s.ctx, false)
				if err != nil || len(feeds) != 2 || p.loads != 0 || syncPlayer(t, &m).Current != nil {
					t.Fatal("search preview persisted, subscribed or played", feeds, err)
				}
				if apple.calls != 0 || index.calls != 0 {
					t.Fatal("URL was sent to directory", apple.calls, index.calls)
				}
			})
		})
	}
}

func TestSearchRejectsInvalidURLWithoutDirectoryDispatch(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		directory := &fixtureDirectory{}
		m.browse.directory, m.browse.indexDirectory = directory, directory
		for _, address := range []string{
			"https://user:private-secret@example.test/rss",
			"ftp://example.test/rss",
			"file:///etc/passwd",
			"https://",
			"https:example.test/rss",
			"http://example.test/%zz",
		} {
			libraryKey(&m, 'f')
			m.browse.searchApple = true
			m.browse.searchIndex = true
			updateLibrary(&m, tea.PasteMsg{Content: address})
			if cmd := updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil || !m.browse.searching || !m.Model.Busy() {
				t.Fatal("invalid URL left editor or dispatched", address)
			}
			if strings.Contains(m.notice, "private-secret") || directory.calls != 0 {
				t.Fatal("invalid URL leaked or reached directory", m.notice)
			}
			updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		}
	})
}

func TestUnifiedURLInputBoundsCancellationAndStalePreview(t *testing.T) {
	server := urlPreviewServer(t)
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		libraryKey(&m, 'f')
		updateLibrary(&m, tea.PasteMsg{Content: server.URL + "/rss?" + strings.Repeat("x", 5000)})
		if len(m.browse.input) != 4096 {
			t.Fatal("unbounded URL input", len(m.browse.input))
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
		updateLibrary(&m, tea.PasteMsg{Content: strings.Repeat("界", 200)})
		if len([]rune(m.browse.input)) != 128 {
			t.Fatal("directory query lost rune-safe bound")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		libraryKey(&m, 'f')
		updateLibrary(&m, tea.PasteMsg{Content: server.URL + "/site"})
		pending := updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		finishLibrary(&m, pending)
		if m.browse.preview != nil || m.browse.previewCancel != nil {
			t.Fatal("canceled unified preview applied")
		}
		if feeds, err := s.store.Feeds(s.ctx, false); err != nil || len(feeds) != 2 {
			t.Fatal("cancel changed cached library", feeds, err)
		}
	})
}
