package ui

// Icon is a closed vocabulary: untrusted metadata never selects a glyph or
// supplies escape sequences. Labels remain readable in every font mode.
type Icon uint8

const (
	ShowIcon Icon = iota
	EpisodeIcon
	SearchIcon
	FeedIcon
	PlayingIcon
	PausedIcon
	StoppedIcon
	FailedIcon
	QueueIcon
	FavoriteIcon
)

var unicodeIcons = [...]string{"◉", "♪", "⌕", "↗", "▶", "Ⅱ", "■", "!", "○", "♥"}
var nerdIcons = [...]string{"\uf2ce", "\uf001", "\uf002", "\uf09e", "\uf04b", "\uf04c", "\uf04d", "\uf071", "\uf017", "\uf004"}

func (m Model) WithIcons(mode string) Model { m.icons = mode; return m }
func (m Model) Icon(icon Icon, label string) string {
	if icon > FavoriteIcon || m.icons == "plain" {
		return label
	}
	glyph := ""
	switch m.icons {
	case "", "unicode":
		glyph = unicodeIcons[icon]
	case "nerd":
		glyph = nerdIcons[icon]
	}
	if glyph == "" {
		return label
	}
	return glyph + " " + label
}
func panelIcon(panel Panel) Icon {
	switch panel {
	case Podcasts:
		return ShowIcon
	case Episodes, Details:
		return EpisodeIcon
	case Favorites:
		return FavoriteIcon
	default:
		return QueueIcon
	}
}
func stateIcon(state string) Icon {
	switch state {
	case "playing":
		return PlayingIcon
	case "paused":
		return PausedIcon
	case "failed":
		return FailedIcon
	default:
		return StoppedIcon
	}
}
