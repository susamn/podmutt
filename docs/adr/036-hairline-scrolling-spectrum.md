# ADR 036: Calm 1px hairline scrolling spectrum visualizer

Status: implemented for #43.

To eliminate visual disturbance and excessive vertical movement in the Player pane,
the heavy block spectrum is replaced with a calm, compact 1px hairline scrolling spectrum:

1. **Hairline Geometry**:
   - Uses thin 1px hairline Unicode characters (`│`, `╷`, `╵`) of different lengths.
   - Sized at a compact height of 1–2 rows maximum between the header and slider bar.
   - 1-character bar width with clean spacing (`gap = 1` or `2`).

2. **Calm Wave Motion**:
   - Multi-harmonic lateral drift smoothly moves across frequency bands without rapid jitter.
   - Styled with subtle `Cyan` and `Accent` tints.
   - Active during playback, frozen on pause, and resting flat when stopped.
