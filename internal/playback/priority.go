package playback

// nextPendingID selects by publication time without mutating insertion order.
// Unknown/equal dates retain their persisted queue order as a stable tie-breaker.
// Presentation sorts must never be written back to the actor's pending queue.
func (e *Engine) nextPendingID() int64 {
	if len(e.state.Queue) == 0 {
		return 0
	}
	newest := e.state.Queue[0]
	for _, episode := range e.state.Queue[1:] {
		if episode.PublishedAt.After(newest.PublishedAt) {
			newest = episode
		}
	}
	return newest.ID
}
