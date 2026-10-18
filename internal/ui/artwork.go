package ui

// ArtworkSize reserves a small left-hand column inside Details. Compact and
// modal layouts hide art before sacrificing descriptions or player controls.
func (m Model) ArtworkSize() (int, int) {
	if m.compact() || m.Busy() || m.data.Prompt != "" {
		return 0, 0
	}
	width, height := m.detailsSize()
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
