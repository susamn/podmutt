# ADR 013: Bounded artwork sources and image-derived terminal fallback

Status: proposed (loading/cache/ASCII implemented; TUI/Kitty in progress)

## Decision

Keep artwork optional and isolated from transport/library persistence. The
foreground owns an in-memory compressed-byte LRU: 8 MiB, at most 32 images,
24-hour sliding idle expiry. No disk cache or permission-sensitive new paths.
Choose episode art first, then show art after invalid/missing/failing episode
art; duplicate sources are attempted only once. Never change subscriptions or
interrupt playback on artwork failure.

Loads are blocking cancelable operations for future Bubble Tea commands, not
independent background workers. At most two loads/decodes run concurrently;
10-second total deadlines include waiting. Allow HTTP(S) without credentials,
three redirects, and at most 3 MiB decoded HTTP body. Preserve stricter caller
redirect/deadline policies; errors never disclose URLs. Only validated image
bytes enter the LRU. Cache hits produce independent decodes, so mutable returned
images cannot corrupt later views. Cache-budget pressure evicts, not fails.

Decode JPEG/PNG/GIF using stdlib only (GIF's first frame). Check dimensions before
decoding: maximum 4096 per axis and four million pixels. Reject SVG/WebP and
malformed/oversized images rather than shelling out or adding codecs. Decoding
itself is bounded CPU work, not interruptible midway; check cancellation after it.

ASCII samples luminance with a 2:1 terminal-cell aspect ratio. Bound output to
120 columns / 60 rows, honor smaller pane limits, and emit no control sequences.
Missing art gets a plain placeholder. Neither Apple promotional artwork nor raw
feed text is used in this projection.

## Remaining in #13

Wire cancellation/versioning to selection/resize and application shutdown, add
Kitty capability detection plus explicit graphics lifecycle/cleanup, and verify
on a real Kitty terminal. This backend foundation does not complete #13 and
makes no claim that Kitty graphics have been tested. The safe fallback is ready
for that integration; no user-facing artwork is enabled yet.
