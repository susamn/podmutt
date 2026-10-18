# Artwork verification

Automated checks cover bounded malformed/oversized/cache/fallback paths, async
selection cancellation, stale generations, resize without re-fetch, missing-art
placeholders, fixed player geometry, Kitty probe fallback, direct PNG chunking,
virtual-placeholder cell width and serialized/coalesced resource deletion.

## Real Kitty — pending

No interactive Kitty terminal was available during implementation. Do not mark
#13 complete from an injected protocol response or captured escape sequences.

Use a recent Kitty supporting Unicode image placeholders (0.28+) without tmux or
screen. Record Kitty/OS/terminal-cell size and commit SHA, then:

- [ ] Open a local feed with episode art, show-only art and missing/invalid art.
- [ ] Episode art is visible; failing episode art falls back to show art.
- [ ] Rapidly switch episodes/shows/Queue/Details; no old image appears on a new
      target. Transport never starts as a side effect of selecting/loading art.
- [ ] Scroll a long description: image remains in its column; Player never moves.
- [ ] Resize wider/narrower and cross compact breakpoints repeatedly: placement
      stays inside Details, compact clears graphics, return restores art.
- [ ] Open help/search/add/confirmation overlays; no image bleeds through them.
- [ ] Quit normally and via interrupt; no orphan graphics remain after returning
      to the shell, and images outside podmutt were not deleted.
- [ ] Repeat in a non-Kitty terminal and tmux/screen: image-derived ASCII/missing
      placeholder only, no malformed APC output, ordinary navigation intact.

The graphics probe plus terminal-version gate require Kitty 0.28+; that is still
not a substitute for checking each actual terminal's placeholder behavior. If a positive probe still renders placeholders incorrectly, record it
and tighten capability handling before marking the manual check complete.
