# ADR 023: Search, charts and direct RSS are different entry points

Status: implemented for #23.

The Podcasts pane advertises f Search podcasts, U RSS/website preview and t Apple
US top 25. Actions are not fake subscriptions/results: list identity, filtering,
selection and paging remain unchanged. Hint rows reserve viewport space while
leaving a selectable item; at minimum sizes the footer prioritizes Search/RSS.

f opens an explicitly labelled search dialog: Apple online shows beyond the chart,
or offline subscribed show/cached episode text (Tab switches scope). The current
context supplies the default. The query/source remain in result headings/details;
loading, empty and error states are distinct. t requests the chart separately.
New requests discard old displayed results and cancel generation-bound work.
Esc cancels in-flight directory/feed previews; stale replies cannot restore them.

U opens an HTTP(S) URL dialog and resolves a feed or website to the existing
read-only preview. URLs with credentials are rejected before dispatch; input is
bounded and the visible tail remains usable for long addresses. No database
writes, subscriptions, queue changes, audio requests or autoplay happen while
previewing. Space/a/A keep explicit ad-hoc playback/enqueue; u subscribes. The
existing N shortcut is explicitly add-and-subscribe, not a preview operation.

Tests use local RSS/website fixtures and cover preview→ad-hoc play→subscribe,
no implicit persistence/audio, input/cancel/stale results, chart/search/error
states, source labels, compact action visibility and width/selection safety.
No new dependency or changes to network/artwork/playback ownership policy.
