package ui

// Geometry is shared by rendering, paging, artwork, reading and animation.
const playerBaseRows = 5 // borders, one identity line, timeline, transport
type wideGeometry struct {
	libraryWidth, episodeWidth, queueWidth, playerWidth int
	episodes, details, player, queue                    int
}

func (m Model) wideLayout() wideGeometry {
	available := max(0, m.height-1)
	g := wideGeometry{libraryWidth: min(28, m.width/5), queueWidth: m.width * 35 / 100}
	g.playerWidth = m.width - g.libraryWidth
	g.episodeWidth = g.playerWidth - g.queueWidth
	g.episodes = available / 2
	// Player borrows only the bottom of Details, not Episodes. Keep at least
	// two interior Details rows for pinned identity and readable overflow.
	g.player = min(max(0, available-g.episodes-4), playerBaseRows+len(m.playerNotices(g.playerWidth-2)))
	g.details = available - g.episodes - g.player
	if m.decoration && g.details >= 10 {
		g.player += 2
		g.details -= 2
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
	player = min(max(0, available-4), playerBaseRows+len(m.playerNotices(m.width-2)))
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
