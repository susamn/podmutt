# ADR 019: Explicit playback does not imply subscription

Status: accepted for post-v1 #21; playback-stack behavior depends on #20.

Discovery/feed preview remains read-only until explicit Space or enqueue. Those
actions capture the selected preview indices, validate feed/enclosure URLs and
cache only selected episodes. The shared actor then plays/enqueues stable local
IDs. Existing GUID/enclosure identities and history are reused. Same-current
Space retains play/pause toggle semantics; Player Space remains current toggle.

Ad-hoc caching does not subscribe, modify existing feed validators, auto-refresh
an unsubscribed feed or fetch audio itself. New ad-hoc feeds have empty validators
because caching selected episodes does not mean the complete feed was persisted.
Existing subscribed feeds retain subscription/validators. Invalid selections are
rejected before writing. Errors do not dispatch playback. User actions retain the
captured identity even if browsing changes before the asynchronous result.

Later explicit subscription of an ad-hoc feed must cache the complete validated
preview before committing validators/subscription; the former existing-URL fast
path would otherwise leave a one-episode cache. Upserts preserve IDs/history.
Already-subscribed feeds keep their existing cache. No new network dependency,
account, auto-subscription or private URL protocol is introduced.

Tests cover GUID/enclosure identity, resume/history, read-only browsing, explicit
play/enqueue without subscription, filtered/bulk selection, invalid/stale previews,
storage errors, validator preservation and later full-preview subscription.
