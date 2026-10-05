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
