package app

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/activity"
	"podmutt/internal/feedback"
	"podmutt/internal/playback"
)

type latestQueued struct{ err error }

func (m libraryShell) queueLatest(feedID int64) tea.Cmd {
	engine, ctx := m.s.playback, m.s.ctx
	return trackedWork(ctx, "Updating latest episodes…", activity.Background, func(ctx context.Context) tea.Msg { return latestQueued{engine.QueueLatest(ctx, feedID)} })
}
func subscriptionEdited(ctx context.Context, engine *playback.Engine, id int64, added bool, err error) libraryEdited {
	if err == nil {
		if queueErr := engine.QueueLatest(ctx, id); queueErr != nil {
			err = &feedback.Fault{Code: feedback.Partial, Safe: "Subscription change saved, but latest episodes could not be queued.", Cause: queueErr}
		}
	}
	return libraryEdited{id: id, added: added, err: err}
}
