package playback

import (
	"context"
)

// RefreshChapters imports only cached metadata for a refreshed feed into the
// actor's current/queue snapshots. Never change the transport or replace the
// metadata for audio that was swapped while an older URL is still playing.
func (e *Engine) RefreshChapters(ctx context.Context, feedID int64) error {
	_, err := e.submit(ctx, func(ctx context.Context) error {
		update := func(episodeID int64, audioURL string) (string, string, string, bool, error) {
			fresh, err := e.repo.Episode(ctx, episodeID)
			if err != nil {
				return "", "", "", false, err
			}
			if fresh.FeedID != feedID || fresh.AudioURL != audioURL {
				return "", "", "", false, nil
			}
			return fresh.ChaptersURL, fresh.ChaptersJSON, fresh.ChaptersState, true, nil
		}
		if current := e.state.Current; current != nil && current.FeedID == feedID {
			url, data, state, ok, err := update(current.ID, current.AudioURL)
			if err != nil {
				return err
			}
			if ok {
				current.ChaptersURL, current.ChaptersJSON, current.ChaptersState = url, data, state
			}
		}
		for i := range e.state.Queue {
			item := &e.state.Queue[i]
			if item.FeedID != feedID {
				continue
			}
			url, data, state, ok, err := update(item.ID, item.AudioURL)
			if err != nil {
				return err
			}
			if ok {
				item.ChaptersURL, item.ChaptersJSON, item.ChaptersState = url, data, state
			}
		}
		return nil
	})
	return err
}
