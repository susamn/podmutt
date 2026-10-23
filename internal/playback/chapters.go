package playback

import (
	"context"

	"podmutt/internal/activity"
	"podmutt/internal/chapters"
)

func (e *Engine) discoverEmbedded(id int64, playbackID uint64) {
	ctx, cancel := context.WithTimeout(e.ctx, operationTimeout)
	defer cancel()
	ctx, finish := activity.Begin(ctx, "Checking embedded chapters…", activity.Background)
	defer finish()
	_, _ = e.EmbeddedChapters(ctx, id, playbackID)
}

type embeddedChapterReader interface {
	ChapterList(context.Context) ([]byte, error)
}
type embeddedChapterCache interface {
	CacheEmbeddedChapters(context.Context, int64, string, string) (bool, error)
	CacheEmbeddedStatus(context.Context, int64, string, string) (bool, error)
}

// EmbeddedChapters inspects only the currently loaded audio, without loading
// media just to discover metadata. Feed metadata wins over embedded chapters.
func (e *Engine) EmbeddedChapters(ctx context.Context, episodeID int64, playbackID uint64) ([]chapters.Chapter, error) {
	state, err := e.submit(ctx, func(ctx context.Context) error {
		if episodeID <= 0 || playbackID == 0 || !e.active || !e.ready || e.state.Current == nil ||
			e.state.Current.ID != episodeID || e.playbackID != playbackID {
			return ErrSeekUnavailable
		}
		if e.state.Current.ChaptersJSON != "" {
			return nil
		}
		if e.state.Current.ChaptersURL != "" {
			return ErrSeekUnavailable
		}
		if e.state.Current.ChaptersState == "embedded-empty" || e.state.Current.ChaptersState == "embedded-error" {
			return nil
		}
		reader, ok := e.player.(embeddedChapterReader)
		var list []chapters.Chapter
		var parseErr error
		if ok {
			var data []byte
			data, parseErr = reader.ChapterList(ctx)
			if parseErr == nil {
				list, parseErr = chapters.FromMPV(data)
			}
		}
		if parseErr != nil || len(list) == 0 {
			status := "embedded-empty"
			if parseErr != nil {
				status = "embedded-error"
			}
			if cache, ok := e.repo.(embeddedChapterCache); ok {
				saved, err := cache.CacheEmbeddedStatus(ctx, episodeID, e.state.Current.AudioURL, status)
				if err != nil {
					return err
				}
				if !saved {
					return ErrSeekUnavailable
				}
			}
			e.state.Current.ChaptersState = status
			return nil
		}
		encoded, err := chapters.Serialize(list)
		if err != nil {
			return err
		}
		if cache, ok := e.repo.(embeddedChapterCache); ok {
			saved, err := cache.CacheEmbeddedChapters(ctx, episodeID, e.state.Current.AudioURL, encoded)
			if err != nil {
				return err
			}
			if !saved {
				return ErrSeekUnavailable
			}
		}
		e.state.Current.ChaptersJSON, e.state.Current.ChaptersState = encoded, "embedded"
		return nil
	})
	if err != nil || state.Current == nil || state.Current.ID != episodeID || state.PlaybackID != playbackID {
		return nil, err
	}
	return state.Current.ParsedChapters(), nil
}
