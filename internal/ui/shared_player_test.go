package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/theme"
)

func TestWidePlayerBorrowsDetailsBottomAndQueueGainsWidth(t *testing.T) {
	for _, size := range [][2]int{{100, 24}, {140, 40}, {160, 44}, {180, 56}} {
		for _, palette := range []theme.Palette{theme.Default(), theme.SolarizedDark(), theme.Catppuccin()} {
			for _, icons := range []string{"plain", "unicode", "nerd"} {
				for _, decoration := range []bool{false, true} {
					for _, notices := range []bool{false, true} {
						data := Data{Player: Player{HasCurrent: true, Status: "playing", Title: "Current audio", Channel: "RSS show", Position: "12:34", Duration: "48:00"}, Queue: []Item{{ID: "pending", PlainTitle: "Pending work", Channel: "RSS show", DurationSeconds: 100, PositionSeconds: 25}}}
						if notices {
							data.Player.Error = "Action failed " + strings.Repeat("long failure ", 30)
							data.Player.Warning = "Media offline " + strings.Repeat("long warning ", 30)
						}
						m := resize(New(data).WithTheme(palette).WithIcons(icons).WithPlayerDecoration(decoration), size[0], size[1]).WithFocus(Details)
						m = m.WithEpisodeDetails(EpisodeDetails{Key: "pinned", Title: "Pinned identity", Show: "RSS show", Inspecting: true, Description: strings.Repeat("Paragraph to scroll.\n\n", 100)})
						g := m.wideLayout()
						oldQueue := size[0] * 3 / 10
						oldMiddle := size[0] - g.libraryWidth - oldQueue
						if g.queueWidth != size[0]*35/100 || g.queueWidth <= oldQueue || g.episodeWidth != oldMiddle-(g.queueWidth-oldQueue) {
							t.Fatal("width transfer is not shared by Episodes/Details", size, g)
						}
						if g.playerWidth != g.episodeWidth+g.queueWidth || g.episodes != (size[1]-statusRows)/2 || g.episodes+g.details != g.queue || g.details < 4 {
							t.Fatal("inconsistent shared bottom geometry", size, g)
						}
						if w, h := m.detailsSize(); w != g.episodeWidth || h != g.details {
							t.Fatal("Details uses stale geometry", w, h, g)
						}
						view := m.View().Content
						lines := strings.Split(ansi.Strip(view), "\n")
						if len(lines) != size[1] || rowContaining(t, view, "Now playing") != g.queue || rowContaining(t, view, "Details") != g.episodes || rowContaining(t, view, "Queue") != 0 {
							t.Fatal("panels did not share bottom Player", size, g)
						}
						playerHeader := ansi.Cut(lines[g.queue], g.libraryWidth, size[0])
						if !strings.Contains(playerHeader, "Now playing") || !strings.HasSuffix(playerHeader, "╮") || ansi.StringWidth(playerHeader) != g.playerWidth {
							t.Fatal("Player does not span both columns", size, playerHeader)
						}
						for _, needle := range []string{"Pinned identity", "Current audio", "[Pause] [Stop] [Next]", "75%"} {
							if !strings.Contains(ansi.Strip(view), needle) {
								t.Fatal("lost visible identity/transport/percentage", size, needle)
							}
						}
						if notices && (!strings.Contains(ansi.Strip(view), "Action failed") || !strings.Contains(ansi.Strip(view), "Media offline")) {
							t.Fatal("shared Player hid failures", size)
						}
						for _, line := range lines {
							if ansi.StringWidth(line) > size[0] {
								t.Fatal("overflow", size, line)
							}
						}
						m = press(m, "G")
						if m.detailScroll == 0 || rowContaining(t, m.View().Content, "Now playing") != g.queue || !strings.Contains(ansi.Strip(m.View().Content), "Pinned identity") {
							t.Fatal("reading moved Player or lost pin", size)
						}
					}
				}
			}
		}
	}
}
