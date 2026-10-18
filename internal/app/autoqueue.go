package app

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/playback"
)

type latestQueued struct{ err error }

func (m libraryShell) queueLatest(feedID int64) tea.Cmd {
	engine, ctx := m.s.playback, m.s.ctx
	return func() tea.Msg { return latestQueued{engine.QueueLatest(ctx, feedID)} }
}
func subscriptionEdited(ctx context.Context, engine *playback.Engine, id int64, added bool, err error) libraryEdited {
	if err == nil {
		if queueErr := engine.QueueLatest(ctx, id); queueErr != nil {
			err = fmt.Errorf("subscription saved; latest episode could not be queued: %w", queueErr)
		}
	}
	return libraryEdited{id: id, added: added, err: err}
}
