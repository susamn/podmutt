package ui

import (
	"strings"

	"podmutt/internal/feedback"
)

// Errors are independent of informational notices/activity so a background
// completion cannot hide a failed user action. Success clears only its scope.
func (m Model) WithFeedback(message feedback.Message) Model {
	if m.message == message {
		return m
	}
	m.message = message
	m.messageScroll = 0
	return m
}
func (m Model) ClearFeedback(scope string) Model {
	if m.message.Scope == scope {
		m.message = feedback.Message{}
		m.messageOpen = false
		m.messageScroll = 0
	}
	return m
}
func (m Model) WithBackgroundFeedback(message feedback.Message) Model {
	m.backgroundMessage = message
	if m.activeMessage().Summary == "" {
		m.messageOpen = false
	}
	return m
}
func (m Model) activeMessage() feedback.Message {
	if m.message.Summary != "" {
		return m.message
	}
	if m.backgroundMessage.Summary != "" {
		return m.backgroundMessage
	}
	if m.data.Player.Warning != "" {
		return feedback.Message{Scope: "Media controls", Severity: "warning", Summary: "Media controls unavailable.", Action: m.data.Player.Warning}
	}
	return feedback.Message{}
}
func (m *Model) scrollFeedback(delta int) {
	width, height := min(72, m.width-2), min(22, m.height-statusRows-2)
	capacity := max(1, height-4)
	m.messageScroll = max(0, min(m.messageScroll+delta, len(m.messageRows(width))-capacity))
}
func (m Model) messageRows(width int) []string {
	message := m.activeMessage()
	text := message.Summary + "\n\n" + message.Action
	if m.data.Player.Warning != "" && message.Scope != "Media controls" {
		text += "\n\nMedia controls:\n" + m.data.Player.Warning
	}
	return wrapDetails(text, max(1, width-4))
}
func (m Model) feedbackModal(width, height int) string {
	rows := m.messageRows(width)
	capacity := max(1, height-4)
	offset := min(m.messageScroll, max(0, len(rows)-capacity))
	end := min(len(rows), offset+capacity)
	body := append([]string(nil), rows[offset:end]...)
	body = append(body, "", "j/k scroll · Esc dismiss")
	title := "Message · !/Esc dismiss"
	if m.activeMessage().Severity == "error" {
		title = "Error · !/Esc dismiss"
	}
	return m.box(title, body, width, min(height, len(body)+2), true, "", "")
}
func (m Model) feedbackStatus() string {
	message := m.activeMessage()
	if message.Scope == "Media controls" && m.message.Summary == "" && m.backgroundMessage.Summary == "" {
		return ""
	}
	if message.Summary == "" {
		return ""
	}
	severity := "Notice"
	if message.Severity == "error" {
		severity = "Error"
	} else if message.Severity == "warning" {
		severity = "Warning"
	}
	return severity + " · ! details · " + clean(strings.TrimSpace(message.Summary+" "+message.Action), false)
}
