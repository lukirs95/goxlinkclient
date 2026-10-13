package xlinkclient

import (
	"context"
	"sync"
)

// latest delivers values to a channel that may be shared with other clients
// and read by a slow consumer. Putting a value never blocks; if the previous
// value has not been delivered yet, it is replaced. The consumer therefore
// always receives the most recent value, and a slow consumer cannot stall the
// connection that produces the values.
type latest[T any] struct {
	out chan<- T

	mu      sync.Mutex
	value   T
	pending bool
	wake    chan struct{}
}

func newLatest[T any](out chan<- T) *latest[T] {
	return &latest[T]{out: out, wake: make(chan struct{}, 1)}
}

// put stores v as the value to deliver next. It never blocks.
func (l *latest[T]) put(v T) {
	l.mu.Lock()
	l.value, l.pending = v, true
	l.mu.Unlock()

	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// take returns the pending value, if any, and clears it.
func (l *latest[T]) take() (T, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.value, l.pending
	var zero T
	l.value, l.pending = zero, false
	return v, ok
}

// run forwards values to out until ctx is done.
func (l *latest[T]) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.wake:
		}
		v, ok := l.take()
		if !ok {
			continue
		}
		select {
		case l.out <- v:
		case <-ctx.Done():
			return
		}
	}
}

// updateKey identifies the element an Update refers to.
type updateKey struct {
	system SystemID
	kind   UpdateKind
	id     string
}

// updateQueue delivers updates to a channel that may be shared with other
// clients and read by a slow consumer. Putting updates never blocks. An
// update that has not been delivered yet is replaced by a newer one of the
// same element and keeps its position, so the consumer receives the most
// recent state of every element and no element is skipped. A snapshot drops
// the pending updates of its system, which it contains.
type updateQueue struct {
	out chan<- Update

	mu    sync.Mutex
	queue []Update
	head  int
	// pending maps the key of every queued update to its index in queue.
	pending map[updateKey]int
	wake    chan struct{}
}

func newUpdateQueue(out chan<- Update) *updateQueue {
	return &updateQueue{out: out, pending: map[updateKey]int{}, wake: make(chan struct{}, 1)}
}

// put queues updates. It never blocks.
func (q *updateQueue) put(updates ...Update) {
	if len(updates) == 0 {
		return
	}
	q.mu.Lock()
	for _, u := range updates {
		if u.Kind == UpdateSnapshot {
			q.dropSystem(u.System)
		}
		key := updateKey{u.System, u.Kind, u.ID}
		if i, ok := q.pending[key]; ok {
			q.queue[i] = u
			continue
		}
		q.pending[key] = len(q.queue)
		q.queue = append(q.queue, u)
	}
	q.mu.Unlock()

	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// dropSystem removes the pending updates of a system. Removed entries stay in
// queue with Kind zero and are skipped by next.
func (q *updateQueue) dropSystem(sys SystemID) {
	for key, i := range q.pending {
		if key.system == sys {
			q.queue[i] = Update{}
			delete(q.pending, key)
		}
	}
}

// next removes and returns the oldest queued update.
func (q *updateQueue) next() (Update, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.head < len(q.queue) {
		u := q.queue[q.head]
		q.queue[q.head] = Update{}
		q.head++
		if u.Kind != 0 {
			delete(q.pending, updateKey{u.System, u.Kind, u.ID})
			return u, true
		}
	}
	q.queue, q.head = q.queue[:0], 0
	return Update{}, false
}

// run forwards updates to out until ctx is done.
func (q *updateQueue) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.wake:
		}
		for u, ok := q.next(); ok; u, ok = q.next() {
			select {
			case q.out <- u:
			case <-ctx.Done():
				return
			}
		}
	}
}
