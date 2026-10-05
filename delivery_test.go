package xlinkclient

import (
	"context"
	"testing"
	"time"
)

func TestLatestKeepsOnlyNewestValue(t *testing.T) {
	out := make(chan int)
	l := newLatest(out)

	// put must never block, even without a reader.
	for i := 1; i <= 100; i++ {
		l.put(i)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go l.run(ctx)

	select {
	case got := <-out:
		if got != 100 {
			t.Errorf("received %d, want the newest value 100", got)
		}
	case <-time.After(time.Second):
		t.Fatal("no value delivered")
	}

	select {
	case got := <-out:
		t.Errorf("received unexpected second value %d", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestLatestDeliversSubsequentValues(t *testing.T) {
	out := make(chan int)
	l := newLatest(out)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go l.run(ctx)

	for i := 1; i <= 3; i++ {
		l.put(i)
		select {
		case got := <-out:
			if got != i {
				t.Errorf("received %d, want %d", got, i)
			}
		case <-time.After(time.Second):
			t.Fatalf("value %d not delivered", i)
		}
	}
}

func TestLatestStopsOnCancel(t *testing.T) {
	out := make(chan int)
	l := newLatest(out)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		l.run(ctx)
		close(done)
	}()

	l.put(1) // nobody reads, run blocks on the send
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run did not return after cancel")
	}
}
