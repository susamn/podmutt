CREATE TABLE discover_snapshots (
    provider TEXT NOT NULL CHECK (provider IN ('apple')),
    country TEXT NOT NULL DEFAULT '',
    query TEXT NOT NULL DEFAULT '',
    mode TEXT NOT NULL CHECK (mode IN ('chart','search')),
    results_json TEXT NOT NULL CHECK (length(results_json) <= 1048576),
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (provider,country,query,mode)
);
CREATE INDEX discover_snapshots_fetched ON discover_snapshots(fetched_at DESC);
DELETE FROM discovered_shows WHERE source_id=(SELECT id FROM sources WHERE name='podcast_index');

CREATE TABLE discover_view (
    singleton INTEGER PRIMARY KEY CHECK (singleton=1),
    provider TEXT NOT NULL CHECK (provider IN ('apple','podcast_index')),
    country TEXT NOT NULL DEFAULT '',
    query TEXT NOT NULL DEFAULT '',
    mode TEXT NOT NULL CHECK (mode IN ('chart','search')),
    selected_feed_url TEXT NOT NULL DEFAULT '',
    updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);
