package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/theme"
)

func TestCompactPlayerGeometrySingleTimelineAndFailurePriority(t *testing.T) {
	for _, size := range [][2]int{{140, 40}, {160, 60}, {100, 24}, {70, 20}, {30, 12}, {30, 14}} {
		for _, status := range []string{"playing", "paused", "stopped", "failed"} {
			for _, notices := range []bool{false, true} {
				player := Player{Status: status, HasCurrent: true, Title: "Current audio", Channel: "RSS show", Position: "12:34", Duration: "48:00", PositionSeconds: 754, DurationSeconds: 2880, Progress: .26}
				if notices {
					player.Error, player.Warning = "Action failed", "Media offline"
				}
				m := resize(New(Data{Player: player, Details: strings.Repeat("Reading another episode\n", 100)}), size[0], size[1]).WithFocus(Details)
				text := ansi.Strip(m.View().Content)
				if strings.Count(text, "12:34") != 1 {
					t.Fatal("duplicated elapsed", size, text)
				}
				for _, needle := range []string{"Current audio", "48:00", "[Stop]", "[Next]"} {
					if !strings.Contains(text, needle) {
						t.Fatal("lost core player field", size, needle, text)
					}
				}
				if notices && (!strings.Contains(text, "Action failed") || !strings.Contains(text, "Media offline")) {
					t.Fatal("lost failures", size, text)
				}
				if len(strings.Split(text, "\n")) != size[1] {
					t.Fatal("height expanded", size)
				}
				for _, row := range strings.Split(text, "\n") {
					if ansi.StringWidth(row) > size[0] {
						t.Fatal("width overflow", size)
					}
				}
				if !m.compact() {
					_, playerHeight := m.rightCellHeights()
					if rowContaining(t, text, "Queue") != 0 || rowContaining(t, text, "Now playing") != size[1]-statusRows-playerHeight {
						t.Fatal("wide Queue must sit above bottom-anchored Player", size, status, notices, text)
					}
				}
				if !m.compact() && !notices {
					details, height := m.rightCellHeights()
					bottom := (size[1] - statusRows) - (size[1]-statusRows)/2
					if height != min(7, bottom-5) || details != bottom-height {
						t.Fatal("player grows with terminal", size, height, details)
					}
				}
			}
		}
	}
}

func TestCompactPlayerKeepsBoundsAcrossThemesAndIconModes(t *testing.T) {
	for _, palette := range []theme.Palette{theme.Default(), theme.SolarizedDark(), theme.Catppuccin()} {
		for _, icons := range []string{"plain", "unicode", "nerd"} {
			m := resize(New(Data{Player: Player{Status: "playing", HasCurrent: true, Title: strings.Repeat("Title 界 ", 100), Channel: strings.Repeat("Show 界 ", 100)}}), 100, 24).WithTheme(palette).WithIcons(icons)
			for _, row := range strings.Split(m.View().Content, "\n") {
				if ansi.StringWidth(row) > 100 {
					t.Fatal("theme/icon overflow", icons)
				}
			}
		}
	}
}

func TestDecorationIsOptInAndHiddenClocksStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := resize(New(Data{Player: Player{Status: "playing"}}), 140, 44).WithAnimation(ctx, func(context.Context, time.Duration) error { return nil })
	if _, cmd := m.SyncAnimation(); cmd != nil {
		t.Fatal("default decoration scheduled a clock")
	}
	m = m.WithPlayerDecoration(true)
	m, cmd := m.SyncAnimation()
	if cmd == nil {
		t.Fatal("optional decoration missing")
	}
	stale := cmd()
	m = m.WithDialog(&Dialog{Title: "Modal"})
	next, cmd := m.Update(stale)
	m = next.(Model)
	if cmd != nil || m.animation.running {
		t.Fatal("hidden animation kept polling")
	}
	m = m.WithDialog(nil).WithPlayerDecoration(false)
	if m.animationFrame(20) != "" {
		t.Fatal("disabled decoration visible")
	}
	if notices := m.WithPlayerError(strings.Repeat("long error ", 100)).playerNotices(20); len(notices) != 2 || !strings.Contains(ansi.Strip(notices[1]), "…") {
		t.Fatal("unbounded error", notices)
	}
	// Missing duration stays unknown, including extremely narrow helper callers.
	for _, width := range []int{0, 4, 28, 78} {
		line := m.renderSliderLine(Player{Position: "0:42"}, width)
		if ansi.StringWidth(line) > width {
			t.Fatal("timeline helper overflow", width, line)
		}
	}
}
