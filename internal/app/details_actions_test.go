package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/feed"
	"podmutt/internal/player"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestDetailsActionsUsePinnedEpisodeNotUnderlyingSelection(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		first, target := m.rows[0], m.rows[1]
		if err := s.store.Checkpoint(s.ctx, target.ID, 73); err != nil {
			t.Fatal(err)
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		syncPlayer(t, &m)
		openLibraryDestination(&m, allDestination)
		m.Model = m.Model.WithFocus(ui.Episodes).Select(ui.Episodes, fmt.Sprint(target.ID))
		finishLibrary(&m, libraryKey(&m, '4'))
		if m.detailTarget == nil || m.detailTarget.ID != target.ID {
			t.Fatal("direct Details focus did not pin identity")
		}
		m.Model = m.Model.Select(ui.Episodes, fmt.Sprint(first.ID))
		finishLibrary(&m, libraryKey(&m, 'a'))
		if state := syncPlayer(t, &m); state.Current.ID != first.ID || len(state.Queue) != 1 || state.Queue[0].ID != target.ID {
			t.Fatal("enqueue targeted selection/current", state)
		}
		if libraryKey(&m, 'a') != nil || m.notice != "Already in Up next" {
			t.Fatal("duplicate queue action", m.notice)
		}
		for _, completed := range []bool{true, false} {
			finishLibrary(&m, libraryKey(&m, 'c'))
			if history, err := s.store.Playback(s.ctx, target.ID); err != nil || history.Completed != completed {
				t.Fatal("completion lost pinned identity or stale state", history, err)
			}
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if state := syncPlayer(t, &m); state.Current.ID != target.ID || state.Current.PositionSeconds != 73 {
			t.Fatal("resume targeted underlying selection", state)
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if libraryKey(&m, 'a') != nil || m.notice != "Already now playing" {
			t.Fatal("current could be queued", m.notice)
		}
		if p.loads != 2 {
			t.Fatal("Enter toggled/reloaded current", p.loads)
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape}))
		if selected, _ := m.Model.Selected(ui.Episodes); m.Model.Focus() != ui.Episodes || selected.ID != fmt.Sprint(target.ID) {
			t.Fatal("return did not restore inspected identity", selected)
		}
	})
}

func TestPreviewDetailsActionsCaptureIdentityWithoutSubscribing(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		syncPlayer(t, &m)
		current := m.state.Current.ID
		m.browse.active = true
		m.browse.preview = &feed.Resolved{URL: "https://preview.test/rss", Fetched: feed.Fetched{Document: feed.Document{Title: "RSS preview", Episodes: []store.Episode{
			{GUID: "preview-a", Title: "Preview A", AudioURL: "https://preview.test/a"},
			{GUID: "preview-b", Title: "Preview B", AudioURL: "https://preview.test/b"},
		}}}}
		m.renderLibrary()
		m.Model = m.Model.WithFocus(ui.Episodes).Select(ui.Episodes, "preview:1")
		libraryKey(&m, 'i')
		m.Model = m.Model.Select(ui.Episodes, "preview:0")
		finishLibrary(&m, libraryKey(&m, 'a'))
		if state := syncPlayer(t, &m); state.Current.ID != current || len(state.Queue) != 1 || state.Queue[0].GUID != "preview-b" {
			t.Fatal("preview enqueue followed another row", state)
		}
		if !strings.Contains(m.View().Content, "In Up next") {
			t.Fatal("cached preview membership not refreshed")
		}
		if libraryKey(&m, 'c') != nil || m.mutating || !strings.Contains(m.notice, "Preview only") {
			t.Fatal("preview changed completion")
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if state := syncPlayer(t, &m); state.Current.GUID != "preview-b" {
			t.Fatal("preview Enter used another target", state)
		}
		feeds, err := s.store.Feeds(s.ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, source := range feeds {
			if source.URL == "https://preview.test/rss" && source.Subscribed {
				t.Fatal("action implicitly subscribed")
			}
		}
		entry, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "end-file", Reason: "eof", PlaylistEntryID: &entry}
		deadline := time.Now().Add(time.Second)
		for syncPlayer(t, &m).Current.ID != current {
			if time.Now().After(deadline) {
				t.Fatal("preview EOF did not advance")
			}
			time.Sleep(time.Millisecond)
		}
		finishLibrary(&m, m.reload())
		if m.detailTarget == nil || !m.detailTarget.Completed || !strings.Contains(m.View().Content, "Completed") {
			t.Fatal("completed unsubscribed preview history became Not started")
		}
		// A replaced preview cannot silently fall back to the newly selected row.
		m.browse.preview.Fetched.Document.Episodes[1].GUID = "changed"
		if updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}) != nil || !strings.Contains(m.notice, "Preview changed") {
			t.Fatal("stale preview fallback")
		}
		m.cancelPreview()
		m.renderLibrary()
		for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: 'a', Text: "a"}, {Code: 'c', Text: "c"}} {
			if updateLibrary(&m, key) != nil {
				t.Fatal("canceled preview action dispatched")
			}
		}
		if p.loads != 3 {
			t.Fatal("canceled preview started audio")
		}
	})
}

func TestInspectedHistoryReadsAreScopedCancelableAndCorrelated(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		target := m.rows[0]
		preview := target
		preview.ID, preview.GUID = 0, ""
		url := m.snapshot.Feeds[0].URL
		if got := readInspected(s.ctx, s.store, m.snapshot.Feeds, &preview, url); got == nil || got.ID != target.ID {
			t.Fatal("preview fallback lost identity", got)
		}
		if got := readInspected(s.ctx, s.store, m.snapshot.Feeds, &preview, "https://other.test/rss"); got != nil {
			t.Fatal("history crossed feeds")
		}
		preview.GUID = "missing"
		if got := readInspected(s.ctx, s.store, m.snapshot.Feeds, &preview, url); got != nil {
			t.Fatal("history crossed GUIDs")
		}
		ctx, cancel := context.WithCancel(s.ctx)
		cancel()
		if readInspected(ctx, s.store, m.snapshot.Feeds, &preview, url) != nil || readInspected(ctx, s.store, m.snapshot.Feeds, &target, "") != nil {
			t.Fatal("canceled read fell back")
		}
		m.Model = m.Model.WithFocus(ui.Details)
		m.detailTarget = &target
		other := m.rows[1]
		other.Completed = true
		updateLibrary(&m, libraryLoaded{version: m.version, snapshot: m.snapshot, details: &other, detailKey: m.detailKey(other)})
		if m.detailTarget.ID != target.ID || m.detailTarget.Completed {
			t.Fatal("late reply changed another inspected episode")
		}
	})
}

func TestDetailsCompletionTracksUnsubscribedHistoryAndGuardsOverlays(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		cached, err := s.store.Episodes(s.ctx, m.snapshot.Feeds[1].ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.Checkpoint(s.ctx, cached[0].ID, 42); err != nil {
			t.Fatal(err)
		}
		openLibraryDestination(&m, continueDestination)
		m.Model = m.Model.WithFocus(ui.Episodes)
		libraryKey(&m, 'i')
		for _, opener := range []rune{'?', 'e'} {
			libraryKey(&m, opener)
			for _, key := range []rune{'a', 'c'} {
				if libraryKey(&m, key) != nil {
					t.Fatal("overlay dispatched episode action")
				}
			}
			updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		}
		for _, completed := range []bool{true, false} {
			finishLibrary(&m, libraryKey(&m, 'c'))
			if m.detailTarget == nil || m.detailTarget.Completed != completed {
				t.Fatal("history dropped from Continue left stale inspection")
			}
		}
		m.detailTarget.ID = 99999
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if p.loads != 0 || syncPlayer(t, &m).Current != nil {
			t.Fatal("missing target fell back to selection")
		}
	})
}
