package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"
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
	AudioURL, ArtworkURL     string
	PublishedAt              time.Time
	DurationSeconds          int64
	PositionSeconds          float64
	Completed                bool
}

func publicURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("store: expected public HTTP(S) URL without credentials")
	}
	return nil
}

// UpsertFeed updates fetched metadata without changing an existing subscription.
// Subscribed applies only to new feeds; use SetSubscribed for user actions.
func (s *Store) UpsertFeed(ctx context.Context, feed Feed) (int64, error) {
	if err := publicURL(feed.URL); err != nil {
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
			if err := publicURL(episode.AudioURL); err != nil {
				return err
			}
			identity := "url:" + episode.AudioURL
			guid := strings.TrimSpace(episode.GUID)
			if guid != "" {
				identity = "guid:" + guid
			}
			var published int64
			if !episode.PublishedAt.IsZero() {
				published = episode.PublishedAt.Unix()
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO episodes
				(feed_id,identity,guid,title,description,audio_url,artwork_url,published_at,duration_seconds)
				VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(feed_id,identity) DO UPDATE SET
				guid=excluded.guid, title=excluded.title, description=excluded.description,
				audio_url=excluded.audio_url, artwork_url=excluded.artwork_url,
				published_at=excluded.published_at, duration_seconds=excluded.duration_seconds`,
				feedID, identity, guid, episode.Title, episode.Description, episode.AudioURL,
				episode.ArtworkURL, published, episode.DurationSeconds)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

const episodeColumns = `e.id,e.feed_id,e.guid,e.title,e.description,e.audio_url,e.artwork_url,
	e.published_at,e.duration_seconds,COALESCE(p.position_seconds,0),COALESCE(p.completed,0)`

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
			&episode.DurationSeconds, &episode.PositionSeconds, &episode.Completed); err != nil {
			return nil, err
		}
		if published != 0 {
			episode.PublishedAt = time.Unix(published, 0).UTC()
		}
		episodes = append(episodes, episode)
	}
	return episodes, rows.Err()
}
