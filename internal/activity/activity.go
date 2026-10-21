// Package activity tracks finite foreground-session work, not idle subscriptions
// or playback clocks. Labels are application-authored; never pass URLs or errors.
package activity

import (
	"context"
	"sort"
	"sync"
	"time"
)

type Priority int

const (
	Background Priority = iota
	Foreground
)

type Operation struct {
	ID               uint64
	Label            string
	Priority         Priority
	Started          time.Time
	Completed, Total int
}
type Snapshot struct {
	Revision   uint64
	Operations []Operation
}
type Tracker struct {
	mu             sync.Mutex
	revision, next uint64
	operations     map[uint64]Operation
	changed        chan struct{}
}
type trackerKey struct{}
type operationKey struct{}
type reference struct {
	tracker *Tracker
	id      uint64
}

func New() *Tracker {
	return &Tracker{operations: make(map[uint64]Operation), changed: make(chan struct{}, 1)}
}
func WithTracker(ctx context.Context, tracker *Tracker) context.Context {
	return context.WithValue(ctx, trackerKey{}, tracker)
}
func FromContext(ctx context.Context) *Tracker {
	if ctx == nil {
		return nil
	}
	tracker, _ := ctx.Value(trackerKey{}).(*Tracker)
	return tracker
}
func (t *Tracker) Changed() <-chan struct{} { return t.changed }
func (t *Tracker) notify() {
	t.revision++
	select {
	case t.changed <- struct{}{}:
	default:
	}
}
func (t *Tracker) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := Snapshot{Revision: t.revision}
	for _, op := range t.operations {
		s.Operations = append(s.Operations, op)
	}
	sort.Slice(s.Operations, func(i, j int) bool {
		a, b := s.Operations[i], s.Operations[j]
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		return a.ID < b.ID
	})
	return s
}

// Begin returns a context that groups nested work into the same operation.
// Cancellation removes the operation even if a worker has not returned yet.
// finish is idempotent and must be deferred, including on error/panic paths.
func Begin(ctx context.Context, label string, priority Priority) (context.Context, func()) {
	t := FromContext(ctx)
	if t == nil || ctx.Err() != nil {
		return ctx, func() {}
	}
	if _, ok := ctx.Value(operationKey{}).(reference); ok {
		return ctx, func() {}
	}
	t.mu.Lock()
	t.next++
	id := t.next
	t.operations[id] = Operation{ID: id, Label: label, Priority: priority, Started: time.Now()}
	t.notify()
	t.mu.Unlock()
	var once sync.Once
	finish := func() {
		once.Do(func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			delete(t.operations, id)
			t.notify()
		})
	}
	stop := context.AfterFunc(ctx, finish)
	return context.WithValue(ctx, operationKey{}, reference{t, id}), func() { stop(); finish() }
}

// Progress is reserved for measurable batches (for example, completed feeds).
func Progress(ctx context.Context, completed, total int) {
	ref, ok := ctx.Value(operationKey{}).(reference)
	if !ok {
		return
	}
	t := ref.tracker
	t.mu.Lock()
	defer t.mu.Unlock()
	op, ok := t.operations[ref.id]
	if !ok {
		return
	}
	op.Total = max(0, total)
	op.Completed = min(max(op.Completed, completed), op.Total)
	t.operations[ref.id] = op
	t.notify()
}
