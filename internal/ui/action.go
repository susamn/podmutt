package ui

// ActionKind identifies a typed user-initiated action forwarded from the UI presentation layer.
type ActionKind string

const (
	ActionToggle       ActionKind = "toggle"
	ActionStop         ActionKind = "stop"
	ActionNext         ActionKind = "next"
	ActionSeekForward  ActionKind = "seek-forward"
	ActionSeekBackward ActionKind = "seek-backward"
	ActionSetTheme     ActionKind = "theme"
	ActionCycleCountry ActionKind = "country-cycle"
)

// Action represents a typed presentation action with optional payload.
type Action struct {
	Kind    ActionKind
	Payload string
}
