package playback

import (
	"context"

	"podmutt/internal/chapters"
)

type chapterCache interface {
	CacheEpisodeChapters(context.Context, int64, string, string) (bool, error)
}
type embeddedChapterReader interface {
	ChapterList(context.Context) ([]byte, error)
}
type embeddedChapterCache interface {
	CacheEmbeddedChapters(context.Context, int64, string) (bool, error)
}

// EmbeddedChapters inspects only the currently loaded audio, without loading
// media just to discover metadata. Feed metadata wins over embedded chapters.
func (e *Engine) EmbeddedChapters(ctx context.Context, episodeID int64, playbackID uint64) ([]chapters.Chapter, error) {
	var list []chapters.Chapter
	_, err := e.submit(ctx, func(ctx context.Context) error {
		if episodeID <= 0 || playbackID == 0 || !e.active || !e.ready || e.state.Current == nil ||
			e.state.Current.ID != episodeID || e.playbackID != playbackID {
			return ErrSeekUnavailable
		}
		if e.state.Current.ChaptersJSON != "" {
			list = e.state.Current.ParsedChapters()
			return nil
		}
		if e.state.Current.ChaptersURL != "" {
			return ErrSeekUnavailable
		}
		reader, ok := e.player.(embeddedChapterReader)
		if !ok {
			return nil
		}
		data, err := reader.ChapterList(ctx)
		if err != nil {
			return err
		}
		list, err = chapters.FromMPV(data)
		if err != nil || len(list) == 0 {
			return err
		}
		encoded, err := chapters.Serialize(list)
		if err != nil {
			return err
		}
		if cache, ok := e.repo.(embeddedChapterCache); ok {
			saved, err := cache.CacheEmbeddedChapters(ctx, episodeID, encoded)
			if err != nil {
				return err
			}
			if !saved {
				return ErrSeekUnavailable
			}
		}
		e.state.Current.ChaptersJSON = encoded
		return nil
	})
	return list, err
}

// CacheChapters installs metadata for precisely the playback and declaration
// that initiated the request. The actor serializes this with Next/Stop/Load;
// storage additionally guards against a concurrent feed refresh changing URLs.
func (e *Engine) CacheChapters(ctx context.Context, episodeID int64, playbackID uint64, url string, list []chapters.Chapter) error {
	if episodeID <= 0 || playbackID == 0 || url == "" || len(list) == 0 {
		return ErrSeekUnavailable
	}
	encoded, err := chapters.Serialize(list)
	if err != nil {
		return err
	}
	_, err = e.submit(ctx, func(ctx context.Context) error {
		if e.state.Current == nil || e.state.Current.ID != episodeID || e.playbackID != playbackID || !e.active || e.state.Current.ChaptersURL != url {
			return ErrSeekUnavailable
		}
		cache, ok := e.repo.(chapterCache)
		if !ok {
			return ErrSeekUnavailable
		}
		saved, err := cache.CacheEpisodeChapters(ctx, episodeID, url, encoded)
		if err != nil {
			return err
		}
		if !saved {
			return ErrSeekUnavailable
		}
		e.state.Current.ChaptersJSON = encoded
		return nil
	})
	return err
}
