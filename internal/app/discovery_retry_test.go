package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"podmutt/internal/discovery"
)

func TestDiscoveryRetryReopensFailedPreviewWithoutSearchingOrFetchingAudio(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/rss" {
			t.Error("unexpected audio request", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if fail.Load() {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, `<rss><channel><title>Show</title><item><title>Episode</title><enclosure url="/audio.mp3"/></item></channel></rss>`)
	}))
	defer server.Close()
	for _, direct := range []bool{false, true} {
		t.Run(fmt.Sprint(direct), func(t *testing.T) {
			fail.Store(true)
			before := calls.Load()
			librarySession(t, func(s *session) {
				directory := &fixtureDirectory{}
				m := browserLibrary(t, s, directory)
				m.browse.active = true
				m.browse.directURL = direct
				m.browse.shows = []discovery.Show{{Title: "Show", FeedURL: server.URL + "/rss"}}
				m.renderLibrary()
				finishLibrary(&m, m.previewShow())
				if !m.browse.retryPreview || !strings.Contains(m.notice, "R to retry this feed preview") {
					t.Fatal(m.notice)
				}
				fail.Store(false)
				finishLibrary(&m, libraryKey(&m, 'R'))
				if m.browse.preview == nil || m.browse.errorText != "" || directory.calls != 0 || calls.Load() != before+2 {
					t.Fatal("wrong retry operation", m.notice, directory.calls, calls.Load())
				}
				m.cancelPreview()
				if m.browse.retryPreview {
					t.Fatal("preview context survived cancellation")
				}
				if direct {
					finishLibrary(&m, libraryKey(&m, 'R'))
					if directory.calls != 0 || m.browse.preview == nil {
						t.Fatal("direct URL retry opened charts")
					}
				}
				finishLibrary(&m, m.searchDirectory("science"))
				if m.browse.retryPreview || m.browse.directURL || directory.query != "science" {
					t.Fatal("search retained old retry target")
				}
			})
		})
	}
}
