package app

import (
	"testing"
	"time"

	"podmutt/internal/feed"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestPreviewSortRetainsDocumentIdentityAndLibraryFocus(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		m.browse.active = true
		m.browse.preview = &feed.Resolved{URL: "https://preview.test/rss", Fetched: feed.Fetched{Document: feed.Document{Title: "Preview", Episodes: []store.Episode{
			{GUID: "older", Title: "Zulu older", AudioURL: "https://preview.test/old", PublishedAt: time.Unix(10, 0)},
			{GUID: "newer", Title: "Alpha newer", AudioURL: "https://preview.test/new", PublishedAt: time.Unix(20, 0)},
		}}}}
		m.Model = m.Model.WithFocus(ui.Podcasts)
		m.renderLibrary()
		if m.Model.Visible(ui.Episodes)[0].ID != "preview:1" {
			t.Fatal("preview not newest first")
		}
		m.Model = m.Model.Select(ui.Episodes, "preview:1")
		libraryKey(&m, 'o')
		if m.Model.Visible(ui.Episodes)[0].ID != "preview:0" || m.Model.Focus() != ui.Podcasts {
			t.Fatal("preview oldest sort changed focus or identity")
		}
		libraryKey(&m, 'o')
		if selected, _ := m.Model.Selected(ui.Episodes); selected.ID != "preview:1" {
			t.Fatal("sort lost episode", selected)
		}
		m.Model = m.Model.WithFocus(ui.Episodes)
		if selected, ok := m.previewEpisode(); !ok || selected.GUID != "newer" {
			t.Fatal("sorted position replaced stable preview identity", selected)
		}
		if m.browse.preview.Fetched.Document.Episodes[0].GUID != "older" || m.browse.preview.Fetched.Document.Episodes[1].ID != 0 || p.loads != 0 {
			t.Fatal("sorting mutated document or played audio")
		}
	})
}
