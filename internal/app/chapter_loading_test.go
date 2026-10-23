package app

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/feed"
	"podmutt/internal/player"
	"podmutt/internal/store"
)

func TestChapterExternalLoadingCachingCancellationAndRecovery(t *testing.T) {
	for _, response := range []struct {
		name, body string
		status     int
	}{
		{"valid", `{"chapters":[{"startTime":0,"title":"Intro"},{"startTime":60,"title":"Discussion"}]}`, 200},
		{"empty", `{"chapters":[]}`, 200},
		{"invalid", `not json`, 200},
		{"http failure", ``, 503},
	} {
		t.Run(response.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(response.status)
				fmt.Fprint(w, response.body)
			}))
			defer server.Close()
			librarySession(t, func(s *session) {
				m, p := playerLibrary(t, s)
				m.refresher = feed.New(s.store, server.Client(), 0)
				selected, _ := m.selectedEpisode()
				selected.ChaptersURL = server.URL + "/chapters.json"
				if err := s.store.UpsertEpisodes(s.ctx, selected.FeedID, []store.Episode{selected}); err != nil {
					t.Fatal(err)
				}
				finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
				entry, _ := p.EntryID(s.ctx)
				p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
				p.events <- player.Event{Name: "file-loaded"}
				deadline := time.Now().Add(time.Second)
				for !syncPlayer(t, &m).CanSeek && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				cmd := libraryKey(&m, 'C')
				if cmd == nil || m.chapterCancel == nil {
					t.Fatal("chapter fetch not started")
				}
				finishLibrary(&m, cmd)
				if calls != 1 {
					t.Fatal("expected one HTTP request", calls)
				}
				switch response.name {
				case "valid":
					if m.Model.ChapterSelection() != 0 {
						t.Fatal("picker not opened", m.notice)
					}
					cached, err := s.store.Episode(s.ctx, selected.ID)
					if err != nil || len(cached.ParsedChapters()) != 2 {
						t.Fatal("valid chapters not persisted", cached, err)
					}
					updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc})
					libraryKey(&m, 'C')
					if calls != 1 || m.Model.ChapterSelection() != 0 {
						t.Fatal("reopening refetched or lost chapters")
					}
				case "empty":
					if !strings.Contains(m.notice, "No chapters found") || m.Model.ChapterSelection() != -1 {
						t.Fatal("empty response feedback", m.notice)
					}
				case "invalid", "http failure":
					if !strings.Contains(m.notice, "Could not load chapters") || m.Model.ChapterSelection() != -1 {
						t.Fatal("failure feedback", m.notice)
					}
				}
				if response.name != "valid" {
					cmd = libraryKey(&m, 'C')
					if cmd == nil {
						t.Fatal("failed load not retryable")
					}
					updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEsc})
					updateLibrary(&m, cmd())
					if m.chapterCancel != nil || m.Model.ChapterSelection() != -1 {
						t.Fatal("late reply resurrected canceled picker")
					}
				}
			})
		})
	}
}

func TestChapterLoadRejectsReplacedCurrent(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		m.chapterGeneration = 5
		m.chapterRequest = chapterRequest{episodeID: 10, playbackID: 2, url: "https://example.test/chapters"}
		m.applyChapters(chapterLoaded{generation: 4, request: m.chapterRequest})
		if m.chapterGeneration != 5 {
			t.Fatal("stale reply changed generation")
		}
		m.applyChapters(chapterLoaded{generation: 5, request: m.chapterRequest, err: errors.New("offline")})
		if m.chapterRequest.episodeID != 0 {
			t.Fatal("reply installed after playback was replaced")
		}
	})
}
