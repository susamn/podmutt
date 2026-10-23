package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"podmutt/internal/chapters"
	"podmutt/internal/httpurl"
)

type Feed struct {
	ID                     int64
	URL, Title, ArtworkURL string
	Subscribed             bool
	ETag, LastModified     string
}

type Episode struct {
	ID, FeedID               int64
	GUID, Title, Description string
	FeedTitle                string
	AudioURL, ArtworkURL     string
	ChaptersURL              string
	ChaptersJSON             string
	ChaptersState            string // external, embedded, inline, empty, error, or unchecked
	PublishedAt              time.Time
	DurationSeconds          int64
	PositionSeconds          float64
	Completed                bool
}

func (e Episode) HasChapters() bool {
	return len(e.ChaptersURL) > 0 || len(e.ChaptersJSON) > 0
}

func (e Episode) ParsedChapters() []chapters.Chapter {
	if len(e.ChaptersJSON) == 0 {
		return nil
	}
	list, _ := chapters.Deserialize(e.ChaptersJSON)
	return list
}

// UpsertFeed updates fetched metadata without changing an existing subscription.
// Subscribed applies only to new feeds; use SetSubscribed for user actions.
func (s *Store) UpsertFeed(ctx context.Context, feed Feed) (int64, error) {
	if _, err := httpurl.Parse(feed.URL); err != nil {
		return 0, err
	}
	var id int64
	err := s.db.QueryRowContext(ctx, `INSERT INTO feeds(url,title,artwork_url,subscribed,etag,last_modified)
		VALUES(?,?,?,?,?,?) ON CONFLICT(url) DO UPDATE SET
		title=excluded.title, artwork_url=excluded.artwork_url,
		etag=excluded.etag, last_modified=excluded.last_modified RETURNING id`,
		feed.URL, feed.Title, feed.ArtworkURL, feed.Subscribed, feed.ETag, feed.LastModified).Scan(&id)
	return id, err
}

func (s *Store) SetSubscribed(ctx context.Context, id int64, subscribed bool) error {
	result, err := s.db.ExecContext(ctx, "UPDATE feeds SET subscribed=? WHERE id=?", subscribed, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) Feeds(ctx context.Context, subscribedOnly bool) ([]Feed, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,url,title,artwork_url,subscribed,etag,last_modified
		FROM feeds WHERE NOT ? OR subscribed=1 ORDER BY title COLLATE NOCASE,id`, subscribedOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var feeds []Feed
	for rows.Next() {
		var feed Feed
		if err := rows.Scan(&feed.ID, &feed.URL, &feed.Title, &feed.ArtworkURL, &feed.Subscribed, &feed.ETag, &feed.LastModified); err != nil {
			return nil, err
		}
		feeds = append(feeds, feed)
	}
	return feeds, rows.Err()
}

// UpsertEpisodes is atomic and preserves IDs/history. Items missing from the
// latest feed remain available. GUID-less items use their enclosure URL identity.
func (s *Store) UpsertEpisodes(ctx context.Context, feedID int64, episodes []Episode) error {
	return transaction(ctx, s.db, func(tx *sql.Tx) error {
		for _, episode := range episodes {
			if _, err := httpurl.Parse(episode.AudioURL); err != nil {
				return err
			}
			identity := EpisodeIdentity(episode)
			guid := strings.TrimSpace(episode.GUID)
			var published int64
			if !episode.PublishedAt.IsZero() {
				published = episode.PublishedAt.Unix()
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO episodes
				(feed_id,identity,guid,title,description,audio_url,artwork_url,published_at,duration_seconds,chapters_url,chapters_json)
				VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(feed_id,identity) DO UPDATE SET
				guid=excluded.guid, title=excluded.title, description=excluded.description,
				audio_url=excluded.audio_url, artwork_url=excluded.artwork_url,
				published_at=excluded.published_at, duration_seconds=excluded.duration_seconds,
				chapters_url=excluded.chapters_url,
				chapters_json=CASE
				WHEN length(excluded.chapters_json)>0 THEN excluded.chapters_json
				WHEN excluded.chapters_url<>episodes.chapters_url OR
					episodes.chapters_origin='inline' OR
					(excluded.audio_url<>episodes.audio_url AND episodes.chapters_origin LIKE 'embedded%') OR
					(excluded.chapters_url='' AND episodes.chapters_origin NOT LIKE 'embedded%') THEN ''
				ELSE episodes.chapters_json END,
				chapters_origin=CASE
				WHEN length(excluded.chapters_json)>0 THEN 'inline'
				WHEN excluded.chapters_url<>episodes.chapters_url OR
					episodes.chapters_origin='inline' OR
					(excluded.audio_url<>episodes.audio_url AND episodes.chapters_origin LIKE 'embedded%') OR
					(excluded.chapters_url='' AND episodes.chapters_origin NOT LIKE 'embedded%') THEN ''
				ELSE episodes.chapters_origin END`,
				feedID, identity, guid, episode.Title, episode.Description, episode.AudioURL,
				episode.ArtworkURL, published, episode.DurationSeconds, episode.ChaptersURL, episode.ChaptersJSON)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

const episodeColumns = `e.id,e.feed_id,e.guid,e.title,e.description,e.audio_url,e.artwork_url,
	e.published_at,e.duration_seconds,COALESCE(p.position_seconds,0),COALESCE(p.completed,0),
 (SELECT title FROM feeds WHERE id=e.feed_id),
 e.chapters_url,e.chapters_json,e.chapters_origin`

const episodeTables = `episodes e LEFT JOIN playback p ON p.episode_id=e.id`

// Episodes returns a show's episodes, or all subscribed episodes when feedID=0.
// Newest first is the default; the presentation layer owns alternate sorts.
func (s *Store) Episodes(ctx context.Context, feedID int64) ([]Episode, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+episodeColumns+` FROM `+episodeTables+`
		JOIN feeds f ON f.id=e.feed_id WHERE (?=0 AND f.subscribed=1) OR e.feed_id=?
		ORDER BY e.published_at DESC,e.id DESC`, feedID, feedID)
	if err != nil {
		return nil, err
	}
	return readEpisodes(rows)
}

// InProgress returns unfinished listening history, including unsubscribed and
// explicitly played preview episodes. Merely queued/unplayed episodes stay out.
func (s *Store) InProgress(ctx context.Context) ([]Episode, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+episodeColumns+` FROM `+episodeTables+`
		WHERE p.completed=0 AND p.position_seconds>0
		ORDER BY e.published_at DESC,e.id DESC`)
	if err != nil {
		return nil, err
	}
	return readEpisodes(rows)
}

// Episode looks up a stable local ID, including unsubscribed preview episodes.
func (s *Store) Episode(ctx context.Context, id int64) (Episode, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+episodeColumns+` FROM `+episodeTables+` WHERE e.id=?`, id)
	if err != nil {
		return Episode{}, err
	}
	episodes, err := readEpisodes(rows)
	if err != nil {
		return Episode{}, err
	}
	if len(episodes) == 0 {
		return Episode{}, sql.ErrNoRows
	}
	return episodes[0], nil
}

func readEpisodes(rows *sql.Rows) ([]Episode, error) {
	defer rows.Close()
	var episodes []Episode
	for rows.Next() {
		var episode Episode
		var published int64
		if err := rows.Scan(&episode.ID, &episode.FeedID, &episode.GUID, &episode.Title,
			&episode.Description, &episode.AudioURL, &episode.ArtworkURL, &published,
			&episode.DurationSeconds, &episode.PositionSeconds, &episode.Completed, &episode.FeedTitle,
			&episode.ChaptersURL, &episode.ChaptersJSON, &episode.ChaptersState); err != nil {
			return nil, err
		}
		if published != 0 {
			episode.PublishedAt = time.Unix(published, 0).UTC()
		}
		episodes = append(episodes, episode)
	}
	return episodes, rows.Err()
}

// ChapterFile identifies a declared external file that has not yet been checked.
type ChapterFile struct {
	EpisodeID int64
	URL       string
}

// PendingChapterFiles omits cached successes, empty files, and prior errors.
// URL changes reset chapters_origin in UpsertEpisodes, making the new file pending.
func (s *Store) PendingChapterFiles(ctx context.Context, feedID int64) ([]ChapterFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,chapters_url FROM episodes
		WHERE feed_id=? AND chapters_url<>'' AND chapters_json='' AND chapters_origin=''
		ORDER BY id`, feedID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []ChapterFile
	for rows.Next() {
		var file ChapterFile
		if err := rows.Scan(&file.EpisodeID, &file.URL); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

// MarkChaptersChecked records an empty or failed external chapter file, so
// scheduled refreshes do not retry a permanently broken URL without user action.
func (s *Store) MarkChaptersChecked(ctx context.Context, episodeID int64, url, state string) error {
	if state != "empty" && state != "error" {
		return fmt.Errorf("invalid chapter state")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE episodes SET chapters_origin=? WHERE id=? AND chapters_url=? AND chapters_json=''`, state, episodeID, url)
	return err
}

// CacheEpisodeChapters saves an external file only while its declaration still
// matches. A concurrent feed refresh changing the URL cannot install stale data.
func (s *Store) CacheEpisodeChapters(ctx context.Context, episodeID int64, url, chaptersJSON string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE episodes SET chapters_json=?,chapters_origin='external' WHERE id=? AND chapters_url=?`, chaptersJSON, episodeID, url)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

// CacheEmbeddedChapters only fills an episode without a feed chapter declaration.
func (s *Store) CacheEmbeddedChapters(ctx context.Context, episodeID int64, audioURL, chaptersJSON string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE episodes SET chapters_json=?,chapters_origin='embedded' WHERE id=? AND audio_url=? AND chapters_url='' AND chapters_json=''`, chaptersJSON, episodeID, audioURL)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

// CacheEmbeddedStatus remembers a checked container without usable chapters.
// RSS refreshes retain the result until the audio URL changes.
func (s *Store) CacheEmbeddedStatus(ctx context.Context, episodeID int64, audioURL, status string) (bool, error) {
	if status != "embedded-empty" && status != "embedded-error" {
		return false, fmt.Errorf("invalid embedded chapter state")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE episodes SET chapters_origin=? WHERE id=? AND audio_url=? AND chapters_url='' AND chapters_json=''`, status, episodeID, audioURL)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

// UpdateEpisodeChapters updates the cached chapters JSON string for an episode.
func (s *Store) UpdateEpisodeChapters(ctx context.Context, episodeID int64, chaptersJSON string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE episodes SET chapters_json=? WHERE id=?", chaptersJSON, episodeID)
	return err
}
