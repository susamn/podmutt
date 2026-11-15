package ui

// ArtworkSize reserves a small left-hand column inside Details. Compact and
// modal layouts hide art before sacrificing descriptions or player controls.
func (m Model) ArtworkSize() (int, int) {
	if m.compact() || m.Busy() || m.data.Prompt != "" || (m.data.EpisodeDetails == nil && m.data.Details == "") {
		return 0, 0
	}
	width, height := m.detailsSize()
	if m.data.EpisodeDetails != nil {
		if width < 78 || height < 12 {
			return 0, 0
		}
		return 16, min(6, height-3)
	}
	return min(24, max(0, (width-3)/3)), min(10, max(0, height-3))
}

// WithArtwork accepts only trusted, generated ASCII/Kitty placeholder rows, not
// feed text. Ordinary details continue through the metadata sanitizer.
func (m Model) WithArtwork(columns int, rows []string) Model {
	m.artColumns = columns
	m.artRows = append([]string(nil), rows...)
	m.scrollDetails(0)
	return m
}
func (m Model) descriptionWidth(width int) int {
	if columns, _ := m.ArtworkSize(); columns > 0 && m.artColumns > 0 {
		return max(1, width-m.artColumns-1)
	}
	return width
}

// ArtworkOrigin returns the zero-based screen cell of the artwork's top-left
// corner (Details pane origin + border + two spaces of padding) plus the
// screen size, for sixel placement. ok is false when art has no fixed place
// (compact layout / too small).
func (m Model) ArtworkOrigin() (x, y, screenW, screenH int, ok bool) {
	if m.compact() || m.width < 30 || m.height < 12 {
		return 0, 0, 0, 0, false
	}
	g := m.wideLayout()
	return g.libraryWidth + 3, g.episodes + 1, m.width, m.height, true
}
