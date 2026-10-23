package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/feed"
	"podmutt/internal/player"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestChapterPrefetchVisibleDuringPlaybackWithoutPickerNetwork(t *testing.T) {
	for _, response := range []struct {
		name, body, expected string
		status               int
	}{
		{"valid", `{"chapters":[{"startTime":0,"title":"Intro"},{"startTime":60,"title":"Discussion"}]}`, "Discussion", 200},
		{"empty", `{"chapters":[]}`, "No chapters", 200},
		{"invalid", `not json`, "unavailable", 200},
		{"http failure", ``, "unavailable", 503},
	} {
		t.Run(response.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/chapters.json" {
					calls.Add(1)
					w.WriteHeader(response.status)
					fmt.Fprint(w, response.body)
					return
				}
				if r.URL.Path == "/feed" {
					fmt.Fprint(w, `<rss version="2.0" xmlns:podcast="http://podcastindex.org/namespace/1.0"><channel><title>Show</title><item><guid>a</guid><title>Episode</title><enclosure url="https://example.test/z" type="audio/mpeg"/><podcast:chapters url="/chapters.json"/></item></channel></rss>`)
					return
				}
				http.NotFound(w, r)
			}))
			defer server.Close()
			librarySession(t, func(s *session) {
				m, p := playerLibrary(t, s)
				m.refresher = feed.New(s.store, server.Client(), 0)
				items, err := s.store.Episodes(s.ctx, m.snapshot.Feeds[0].ID)
				if err != nil {
					t.Fatal(err)
				}
				var selected store.Episode
				for _, ep := range items {
					if ep.GUID == "a" {
						selected = ep
					}
				}
				if selected.ID == 0 {
					t.Fatal("missing fixture episode")
				}
				feedID := selected.FeedID
				m.Model = m.Model.Select(ui.Episodes, fmt.Sprint(selected.ID))
				// Play before the feed refresh, so actor metadata is deliberately stale.
				finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
				entry, _ := p.EntryID(s.ctx)
				p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
				p.events <- player.Event{Name: "file-loaded"}
				deadline := time.Now().Add(time.Second)
				for !syncPlayer(t, &m).CanSeek && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				if result := m.refresher.Refresh(s.ctx, store.Feed{ID: feedID, URL: server.URL + "/feed", Title: "Show"}); result.Error != nil {
					t.Fatal(result.Error)
				}
				if calls.Load() != 1 {
					t.Fatal("file not prefetched", calls.Load())
				}
				_ = libraryKey(&m, 'C')
				if response.name == "valid" {
					if m.Model.ChapterSelection() != 0 || !strings.Contains(m.View().Content, response.expected) {
						t.Fatal("cached chapters missing", m.notice)
					}
					updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc})
					libraryKey(&m, 'C')
					if m.Model.ChapterSelection() != 0 {
						t.Fatal("cached reopen failed")
					}
				} else if !strings.Contains(m.notice, response.expected) {
					t.Fatal("missing cached failure feedback", m.notice)
				}
				if calls.Load() != 1 {
					t.Fatal("C refetched file", calls.Load())
				}
			})
		})
	}
}
