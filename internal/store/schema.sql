CREATE TABLE feeds (
    id INTEGER PRIMARY KEY,
    url TEXT NOT NULL UNIQUE CHECK (length(url) > 0),
    title TEXT NOT NULL DEFAULT '',
    artwork_url TEXT NOT NULL DEFAULT '',
    subscribed INTEGER NOT NULL DEFAULT 0 CHECK (subscribed IN (0, 1)),
    etag TEXT NOT NULL DEFAULT '',
    last_modified TEXT NOT NULL DEFAULT ''
);

CREATE TABLE episodes (
    id INTEGER PRIMARY KEY,
    feed_id INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
    identity TEXT NOT NULL CHECK (length(identity) > 0),
    guid TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    audio_url TEXT NOT NULL CHECK (length(audio_url) > 0),
    artwork_url TEXT NOT NULL DEFAULT '',
    published_at INTEGER NOT NULL DEFAULT 0,
    duration_seconds INTEGER NOT NULL DEFAULT 0 CHECK (duration_seconds >= 0),
    UNIQUE (feed_id, identity)
);
CREATE INDEX episodes_by_date ON episodes(published_at DESC, id DESC);

CREATE TABLE playback (
    episode_id INTEGER PRIMARY KEY REFERENCES episodes(id) ON DELETE CASCADE,
    position_seconds REAL NOT NULL DEFAULT 0 CHECK (position_seconds >= 0),
    completed INTEGER NOT NULL DEFAULT 0 CHECK (completed IN (0, 1))
);

CREATE TABLE current_episode (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    episode_id INTEGER NOT NULL REFERENCES episodes(id) ON DELETE CASCADE
);

CREATE TABLE queue (
    episode_id INTEGER PRIMARY KEY REFERENCES episodes(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL UNIQUE CHECK (ordinal >= 0)
);
