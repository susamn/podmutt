package app

import (
	"strings"

	"podmutt/internal/ui"
)

// The popup is a snapshot of the active episode, never the highlighted row.
// Only stored URLs are shown: GUIDs and links embedded in chapter data are not URLs.
func (m *libraryShell) openCurrentURLs() {
	var lines []string
	if current := m.state.Current; current != nil {
		for _, source := range m.snapshot.Feeds {
			if source.ID == current.FeedID {
				lines = append(lines, "Feed")
				lines = appendURL(lines, "RSS", source.URL)
				lines = appendURL(lines, "Artwork", source.ArtworkURL)
				break
			}
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "Episode · "+strings.Join(strings.Fields(current.Title), " "))
		before := len(lines)
		lines = appendURL(lines, "Audio", current.AudioURL)
		lines = appendURL(lines, "Artwork", current.ArtworkURL)
		lines = appendURL(lines, "Chapters", current.ChaptersURL)
		if len(lines) == before {
			lines = append(lines, "  No stored URLs for this episode.")
		}
	} else {
		lines = append(lines, "Nothing playing. Start an episode to see its URLs.")
	}
	m.urlDialog = &ui.Dialog{
		Title:      "Current playback · URLs",
		Body:       strings.Join(lines, "\n"),
		Hint:       "j/k scroll · I/Esc close",
		Scrollable: true,
	}
	m.Model = m.Model.WithDialog(m.urlDialog)
}

func appendURL(lines []string, label, address string) []string {
	if address != "" {
		return append(lines, "  "+label+": "+address)
	}
	return lines
}
