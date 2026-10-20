package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/theme"
)

func TestPlayerIdentityHasOnlyOneSpaceAroundDot(t *testing.T) {
	const title = "The State of Browser Testing"
	const channel = "Software Engineering Daily"
	m := New(Data{})
	for _, width := range []int{70, 100, 180} {
		line := ansi.Strip(m.renderPlayerTopLine(Player{Title: title, Channel: channel}, width))
		if line != "  "+title+" · "+channel+"  " {
			t.Fatalf("short identity must stay adjacent at %d: %q", width, line)
		}
	}
	longTitle := strings.Repeat("Some really long episode name ", 10)
	for _, width := range []int{70, 100, 180} {
		budget := width - 4 - 3 - ansi.StringWidth(channel)
		want := "  " + ansi.Truncate(longTitle, budget, "…") + " · " + channel + "  "
		line := ansi.Strip(m.renderPlayerTopLine(Player{Title: longTitle, Channel: channel}, width))
		if line != want {
			t.Fatalf("truncated title must stay adjacent: got %q want %q", line, want)
		}
	}
	if line := ansi.Strip(m.renderPlayerTopLine(Player{Title: title}, 180)); line != "  "+title+"  " {
		t.Fatalf("channel-free title was padded: %q", line)
	}
}

func TestEpisodeMarkersHaveExactlyOneSpaceBeforeTitle(t *testing.T) {
	for _, palette := range []theme.Palette{theme.Default(), theme.SolarizedDark(), theme.Catppuccin()} {
		for _, icons := range []string{"plain", "unicode", "nerd"} {
			m := New(Data{}).WithTheme(palette).WithIcons(icons)
			for _, sample := range []struct {
				queued, completed bool
				marker            string
			}{{true, false, "+"}, {true, true, "+"}, {false, false, "◉"}, {false, true, "◉"}} {
				for _, width := range []int{30, 56, 70, 100} {
					item := Item{PlainTitle: "Episode", Channel: "Show", Duration: "42:00", Queued: sample.queued, Completed: sample.completed}
					row := ansi.Strip(m.renderEpisodeRow(item, width, false))
					if !strings.HasPrefix(row, "  "+sample.marker+" Episode") || strings.Contains(row, "[ ]") || strings.Contains(row, "[+]") || strings.Contains(row, "[✓]") {
						t.Fatalf("marker must directly precede title: %q", row)
					}
					if episodeColumns(width).gutter != 4 || !strings.HasPrefix(ansi.Strip(m.renderEpisodesHeader(width)), "    Title") {
						t.Fatal("header/marker gutters differ", width)
					}
				}
			}
		}
	}
}
