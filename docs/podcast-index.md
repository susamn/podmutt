# Podcast Index setup and limitations

Podcast Index is optional. Apple search, Apple US top 25 and RSS/website entry work
without API credentials. Press **f**, then **Tab** to choose Library (offline),
Apple or **Podcast Index** explicitly. Enter a nonempty show query. No automatic
provider fallback, account enrollment, retry loop or remote episode search occurs.
The top-25 shortcut **t** always identifies Apple, not Podcast Index.

## Credentials and terms

Obtain **your own** authorized key/secret at <https://api.podcastindex.org/>.
Read/accept the current [official terms](https://github.com/Podcastindex-org/legal/blob/main/TermsOfService.md)
and [API documentation](https://podcastindex-org.github.io/docs-api/) yourself.
Do not use/share another developer's credentials. This client supplies no shared
key and does not imply Podcast Index endorsement or a legal determination that
all possible uses are permitted. Provider access/limits/terms may change.

Set these two environment variables in the process launching podmutt:

- `PODMUTT_PODCAST_INDEX_KEY`
- `PODMUTT_PODCAST_INDEX_SECRET`

Use your local secret manager/environment injection; do not paste actual values
into bug reports, shell history, shared scripts or config committed to Git. Values
are never printed by this integration, persisted in TOML/SQLite or sent to mpv.
Missing credentials produce an actionable error without switching providers.
401/403 means check your access/credentials and system clock, not that RSS is broken.
Native app launchers may not inherit your Terminal environment; launch the bundled
executable from that configured Terminal when checking this optional provider.

## Privacy / data shared

podmutt collects no telemetry. Only when you explicitly submit an Index search,
Podcast Index receives your query, public network address, API key, timestamp,
authentication hash and identifying User-Agent over HTTPS. The secret itself is
not transmitted. Listening history/database/audio selections are not uploaded to
Index. The provider handles requests under its own
[privacy policy](https://github.com/Podcastindex-org/legal/blob/main/PrivacyPolicy.md).
Selecting a result separately contacts its feed/website host; playback/artwork then
use the normal explicit media/artwork paths. Choose offline Library search to
avoid sending a query to a directory. No client-side API response persistence.

## Protocol and bounded behavior

Search uses `/api/1.0/search/byterm?q=...&max=25`, signed with `X-Auth-Key`,
`X-Auth-Date` (Unix UTC seconds) and lowercase SHA-1(key + secret + timestamp) in
`Authorization`, plus an identifying User-Agent. The official
[OpenAPI schema](https://raw.githubusercontent.com/Podcastindex-org/docs-api/master/docs/pi_api.json)
confirms the `feeds` array and these headers; the timestamp window is three minutes. SHA-1 is required by this service's
protocol, not used as a local password-storage design. Correct system time matters.

Each instance serializes requests with a 3-second local gate (not a claim about
provider-granted quota), a 15-second total deadline including quota wait, and a
1MiB JSON limit. All authenticated redirects are blocked; credentials never reach
feed/website hosts. 429 honors bounded Retry-After (up to 30s); no automatic retry.
Errors discard response bodies instead of exposing echoed secrets. Apple and Index
share bounded I/O/normalization policy, not response caches or credentials.

Terms include restrictions on permanent API-content copies, caching beyond cache
headers and required attribution. This implementation keeps **no API response
cache**: results are current-view state and discarded when leaving Index. Source
labels/links attribute **Podcast Index**. Selecting a result fetches the original
RSS separately; browsing stays read-only, while explicit Space/a/A/u reuse normal
RSS-derived cache/history/subscription identities. No bulk directory export or
API descriptions/artwork/audio are copied into the library.

## Verification

Deterministic fixtures verify signatures, malformed/oversize/error/rate-limited
responses, blocked redirects, missing credentials, canceled/concurrent requests,
provider selection, stale replies and shared RSS episode identity/ad-hoc playback.
No real API credential is required in CI. A successful live authenticated search
with your own access is **not implied** by fixture tests; verify it separately,
then preview, play without subscribing and explicitly subscribe if desired.
