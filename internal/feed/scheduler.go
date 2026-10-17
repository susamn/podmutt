package feed

import (
	"context"
	"fmt"
	"sync"
	"time"

	"podmutt/internal/store"
)

// RefreshNow coalesces requests while a cycle runs; cycles never overlap.
func (r *Refresher) RefreshNow() {
	select {
	case r.trigger <- struct{}{}:
	default:
	}
}

// Start refreshes immediately and periodically until ctx is canceled. Results
// arrive per feed as work finishes. Consume results or cancel the session.
func (r *Refresher) Start(ctx context.Context) (<-chan Result, error) {
	ticker := time.NewTicker(r.interval)
	results, err := r.start(ctx, ticker.C, ticker.Stop)
	if err != nil {
		ticker.Stop()
	}
	return results, err
}

func (r *Refresher) start(ctx context.Context, ticks <-chan time.Time, stop func()) (<-chan Result, error) {
	if !r.started.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("feed refresh already running")
	}
	results := make(chan Result, parallelism)
	go func() {
		defer close(results)
		defer r.started.Store(false)
		defer stop()
		for {
			r.cycle(ctx, results)
			select {
			case <-ctx.Done():
				return
			case <-ticks:
			case <-r.trigger:
			}
		}
	}()
	return results, nil
}

func (r *Refresher) cycle(ctx context.Context, results chan<- Result) {
	feeds, err := r.repo.Feeds(ctx, true)
	if err != nil {
		select {
		case results <- Result{Error: err}:
		case <-ctx.Done():
		}
		return
	}
	jobs := make(chan store.Feed)
	var workers sync.WaitGroup
	for range min(parallelism, len(feeds)) {
		workers.Go(func() {
			for feed := range jobs {
				result := r.Refresh(ctx, feed)
				select {
				case results <- result:
				case <-ctx.Done():
					return
				}
			}
		})
	}
queue:
	for _, feed := range feeds {
		select {
		case jobs <- feed:
		case <-ctx.Done():
			break queue
		}
	}
	close(jobs)
	workers.Wait()
}
