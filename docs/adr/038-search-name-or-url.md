# ADR 038: Search accepts names or URLs

Status: accepted for #40 (entry-point slice).

`f` accepts directory/library queries or feed/website URLs. Recognizable URL input
uses the existing validated read-only preview path, independent of the selected
source. Invalid/credential-bearing URLs stay in the editor and never become
remote directory queries. Names retain the explicitly selected search scope.

Preserve 128-rune query and 4096-rune URL limits, cancellation, stale-result guards,
and explicit subscribe/play/enqueue. Reuse `U` preview handling; retain `U`/`N`
for compatibility. No dependency, provider fallback, or persistence change.

This extends ADR 023. Provider selection and reducing advertised shortcuts remain
separate work under #40; do not change them implicitly in this slice.
