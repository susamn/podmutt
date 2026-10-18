CREATE INDEX episodes_latest_per_feed ON episodes(feed_id,published_at DESC,identity ASC);

-- A durable receipt prevents refresh/restart from undoing a user's removal.
CREATE TABLE autoqueue_seen (
    episode_id INTEGER PRIMARY KEY REFERENCES episodes(id) ON DELETE CASCADE
);
