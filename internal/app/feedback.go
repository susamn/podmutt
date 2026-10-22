package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"podmutt/internal/feedback"
)

func (m *libraryShell) report(scope string, err error, retry string) {
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}
	message := feedback.For(scope, err)
	if retry != "" {
		message = feedback.Retry(message, retry)
	}
	m.notice = message.Text()
	if message.Severity == "error" || message.Severity == "warning" {
		m.Model = m.Model.WithFeedback(message)
	}
}
func (m *libraryShell) recordBackgroundFailure(scope string, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	m.backgroundError = feedback.For(scope, err)
	m.backgroundError.Severity = "warning"
}
func (m *libraryShell) clearBackgroundFailure(scope string) {
	if m.backgroundError.Scope == scope {
		m.backgroundError = feedback.Message{}
	}
}
func (m libraryShell) backgroundFeedback() feedback.Message {
	message := m.refreshFeedback()
	if message.Summary == "" {
		return m.backgroundError
	}
	if m.backgroundError.Summary != "" {
		message.Action += "\n" + m.backgroundError.Text()
	}
	return message
}
func (m libraryShell) refreshFeedback() feedback.Message {
	if len(m.refreshErrors) == 0 {
		return feedback.Message{}
	}
	total := 0
	for _, f := range m.snapshot.Feeds {
		if f.Subscribed {
			total++
		}
	}
	ids := make([]int64, 0, len(m.refreshErrors))
	for id := range m.refreshErrors {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	lines := []string{"Cached episodes remain available."}
	for _, id := range ids {
		name := "Feed refresh"
		for _, f := range m.snapshot.Feeds {
			if f.ID == id {
				name = feedback.PublicLabel(f.Title)
				if name == "" {
					name = "Untitled show"
				}
				break
			}
		}
		lines = append(lines, name+": "+feedback.For("Feed refresh", m.refreshErrors[id]).Text())
	}
	retry := "Press R in Library to retry feed refresh."
	if m.browse.active {
		retry = "Press b to open Library, then R to retry feed refresh."
	}
	lines = append(lines, retry)
	summary := fmt.Sprintf("%d feed refreshes failed.", len(ids))
	if total > 0 && len(ids) <= total && ids[0] != 0 {
		summary = fmt.Sprintf("%d of %d feeds could not be refreshed.", len(ids), total)
	}
	return feedback.Message{Scope: "Feed refresh", Code: feedback.Unknown, Severity: "warning", Summary: summary, Action: strings.Join(lines, "\n")}
}
