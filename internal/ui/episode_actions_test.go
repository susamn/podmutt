package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestEpisodeActionHintsAreSharedScopedAndBounded(t *testing.T) {
	for _, panel := range []Panel{Episodes, Details} {
		m := resize(New(Data{PodcastLabel: "My library"}), 140, 40).WithFocus(panel).WithEpisodeActions(EpisodeActions{Available: true, Resume: true, CanComplete: true, Completed: true, Queued: true})
		hints := ansi.Strip(m.bottomHints())
		for _, action := range []string{"Enter:resume", "a:queued", "c:unfinished"} {
			if !strings.Contains(hints, action) {
				t.Fatal("inconsistent episode action", panel, action, hints)
			}
		}
	}
	m := resize(New(Data{}), 70, 20).WithFocus(Details).WithEpisodeActions(EpisodeActions{Available: true})
	if hints := ansi.Strip(m.bottomHints()); !strings.Contains(hints, "c:subscribe first") || !strings.Contains(hints, "Esc:back") {
		t.Fatal("preview eligibility not explained", hints)
	}
	m = resize(m.WithEpisodeActions(EpisodeActions{Available: true, Current: true}), 30, 12)
	if hints := ansi.Strip(m.View().Content); !strings.Contains(hints, "Enter:play") || !strings.Contains(hints, "a:current") {
		t.Fatal("small actionable hints hidden", hints)
	}
	m = m.WithEpisodeActions(EpisodeActions{})
	if hints := ansi.Strip(m.bottomHints()); strings.Contains(hints, "Enter:") || strings.Contains(hints, "a:queue") {
		t.Fatal("empty target advertised actions", hints)
	}
}
