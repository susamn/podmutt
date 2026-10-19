package ui

// Geometry is shared by rendering, paging, artwork, reading and animation.
type wideGeometry struct {
	libraryWidth, episodeWidth, queueWidth int
	episodes, details, player, queue       int
}

func (m Model) wideLayout() wideGeometry {
	available := max(0, m.height-1)
	g := wideGeometry{libraryWidth: min(28, m.width/5), queueWidth: m.width * 3 / 10}
	g.episodeWidth = m.width - g.libraryWidth - g.queueWidth
	g.episodes = available / 2
	g.details = available - g.episodes
	g.player = min(max(0, available-5), 6+len(m.playerNotices(g.queueWidth-2)))
	if m.decoration && available-g.player >= 10 {
		g.player += 2
	}
	g.queue = available - g.player
	return g
}

func (m Model) rightCellHeights() (details, player int) {
	g := m.wideLayout()
	return g.details, g.player
}

func (m Model) compactHeights() (top, middle, player int) {
	available := max(0, m.height-1)
	base := 6
	if m.height < 14 {
		base = 5
	}
	player = min(max(0, available-4), base+len(m.playerNotices(m.width-2)))
	return 0, available - player, player
}

func (m Model) detailsSize() (width, height int) {
	if m.compact() {
		_, height, _ = m.compactHeights()
		return m.width, height
	}
	g := m.wideLayout()
	return g.episodeWidth, g.details
}
