CREATE TABLE sources (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL
);

INSERT INTO sources (id, name, display_name) VALUES
    (1, 'apple', 'Apple Podcasts'),
    (2, 'podcast_index', 'Podcast Index');

CREATE TABLE discovered_shows (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    feed_url TEXT UNIQUE NOT NULL CHECK (length(feed_url) > 0),
    artwork_url TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    country TEXT NOT NULL DEFAULT 'us',
    source_id INTEGER NOT NULL REFERENCES sources(id) ON DELETE RESTRICT,
    source_external_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);
CREATE INDEX discovered_shows_country ON discovered_shows(country, updated_at DESC);
CREATE INDEX discovered_shows_title ON discovered_shows(title);

CREATE TABLE show_ratings (
    show_id INTEGER NOT NULL REFERENCES discovered_shows(id) ON DELETE CASCADE,
    source_id INTEGER NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    rating REAL NOT NULL CHECK (rating >= 0 AND rating <= 5),
    review_count INTEGER NOT NULL DEFAULT 0 CHECK (review_count >= 0),
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    PRIMARY KEY (show_id, source_id)
);
