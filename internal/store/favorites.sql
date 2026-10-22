CREATE TABLE favorites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    episode_id INTEGER UNIQUE NOT NULL REFERENCES episodes(id) ON DELETE RESTRICT,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);
CREATE INDEX favorites_episode ON favorites(episode_id);
