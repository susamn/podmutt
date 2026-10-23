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
	reserved := 1
	if height >= 12 {
		reserved += 3
	}
	capacity := max(1, modalBodyCapacity(height)-reserved)
	m.messageScroll = max(0, min(m.messageScroll+delta, len(m.messageRows(width))-capacity))
}
func (m Model) messageRows(width int) []string {
	message := m.activeMessage()
	text := message.Summary + "\n\n" + message.Action
	if m.data.Player.Warning != "" && message.Scope != "Media controls" {
		text += "\n\nMedia controls:\n" + m.data.Player.Warning
	}
	return wrapDetails(text, modalTextWidth(width))
}
func (m Model) feedbackModal(width, height int) string {
	message := m.activeMessage()
	title, label := "Message", "Notice"
	if message.Severity == "error" {
		title, label = "Error", "Action required"
	}
	if message.Severity == "warning" {
		title, label = "Warning", "Attention"
	}
	compact := height < 12
	reserved := 1 // footer
	if !compact {
		reserved += 3
	} // label and two group gaps
	capacity := max(1, modalBodyCapacity(height)-reserved)
	all := m.messageRows(width)
	offset := min(m.messageScroll, max(0, len(all)-capacity))
	end := min(len(all), offset+capacity)
	var rows []string
	if !compact {
		rows = append(rows, m.modalLabel(label, width), "")
	}
	for _, row := range all[offset:end] {
		rows = append(rows, m.modalLine(row, width))
	}
	if !compact {
		rows = append(rows, "")
	}
	hint := "j/k scroll · ! or Esc dismiss"
	if compact {
		hint = "Esc dismiss"
	}
	rows = append(rows, m.modalFooter(hint, width))
	return m.box(title, rows, width, min(height, len(rows)+2), true, "", "")
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
