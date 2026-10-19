package ui

// Shared geometry is used by rendering, scrolling, artwork and decoration.
func (m Model) rightCellHeights() (details, player int) {
	available := max(0, m.height-1)
	bottom := available - available/2
	width := m.width - m.width*2/5
	player = min(max(0, bottom-5), 6+len(m.playerNotices(width-2)))
	if m.decoration && bottom-player >= 10 {
		player += 2
	}
	return bottom - player, player
}

func (m Model) compactHeights() (top, middle, player int) {
	available := max(0, m.height-1)
	base := 6
	if m.height < 14 {
		base = 5
	}
	player = min(max(0, available-4), base+len(m.playerNotices(m.width-2)))
	remaining := available - player
	if remaining >= 10 {
		top = max(4, min(6, remaining/3))
	}
	return top, remaining - top, player
}

func (m Model) detailsSize() (width, height int) {
	if m.compact() {
		_, height, _ = m.compactHeights()
		return m.width, height
	}
	height, _ = m.rightCellHeights()
	return m.width - m.width*2/5, height
}
