# ADR 013: Bounded artwork sources and image-derived terminal fallback

Status: implemented; real Kitty terminal verification pending

## Decision

Keep artwork optional and isolated from transport/library persistence. The
foreground owns an in-memory compressed-byte LRU: 8 MiB, at most 32 images,
24-hour sliding idle expiry. No disk cache or permission-sensitive new paths.
Choose episode art first, then show art after invalid/missing/failing episode
art; duplicate sources are attempted only once. Never change subscriptions or
interrupt playback on artwork failure.

Loads are blocking cancelable operations inside versioned Bubble Tea commands, not
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

## TUI and graphics lifecycle

Details reserves a small left column for selected episode/show art, with the
wrapped/scrollable description alongside. The fixed Player half is unchanged.
Loading is asynchronous and keyed by episode/show source URLs; changing target or
hiding art cancels the pending request and rejects late generations. Resizing
uses the already decoded image, not another download. Failed/missing art gets a
plain placeholder without disrupting transport/history/subscriptions. Cached
feeds and read-only RSS previews both participate; Apple promotional art is not
fetched. Compact and help/input/confirmation overlays hide art first.

Kitty requires a conservative terminal-environment match, no tmux/screen, AND a
positive bounded graphics protocol probe and a Kitty 0.28+ version response.
Unknown/old/negative/timed-out responses use
ASCII. Virtual placements use Unicode placeholders so the ordinary renderer owns
cell positioning/clipping. Small aspect-preserving PNGs are sent directly in
<=4096-byte base64 chunks; no file paths/shared memory/external encoders. Reuse the
existing Charm ANSI/Kitty and input decoder modules, not another image dependency.

One outstanding Raw+ack sequence serializes uploads/deletions and coalesces rapid
selection/resize/pane changes to the latest placement. Only the foreground's
random nonzero 24-bit image ID is deleted, including a final deletion after the
terminal program has shut down; never clear another application's images.

## Remaining verification

Real Kitty rendering, cell aspect and cleanup require the manual checklist in
`docs/artwork-check.md`. Protocol/fixture/PTY tests are not that verification;
#13 remains open until it is recorded.
