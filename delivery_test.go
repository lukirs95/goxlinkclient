package xlinkclient

import (
	"context"
	"fmt"
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

func TestUpdateQueueKeepsNewestPerElement(t *testing.T) {
	out := make(chan Update)
	q := newUpdateQueue(out)
	enc := func(sys SystemID, name string) Update {
		return Update{System: sys, Kind: UpdateEncoder, ID: "E1", Encoder: &Encoder{Name: name}}
	}
	// put never blocks. A newer update of an element replaces the pending
	// one and keeps its position; a snapshot drops the pending updates of
	// its own system only.
	q.put(enc("A", "a1"), Update{System: "A", Kind: UpdateDecoder, ID: "D1"}, enc("B", "b1"))
	q.put(enc("A", "a2"), enc("B", "b2"))
	q.put(Update{System: "A", Kind: UpdateSnapshot, Snapshot: &System{}}, enc("A", "a3"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go q.run(ctx)

	var got []string
	for range 3 {
		select {
		case u := <-out:
			s := fmt.Sprintf("%s %v", u.System, u.Kind)
			if u.Encoder != nil {
				s += " " + u.Encoder.Name
			}
			got = append(got, s)
		case <-time.After(time.Second):
			t.Fatalf("received %v, want three updates", got)
		}
	}
	if want := "[B Encoder b2 A Snapshot A Encoder a3]"; fmt.Sprint(got) != want {
		t.Errorf("received %v, want %s", got, want)
	}
	select {
	case u := <-out:
		t.Errorf("unexpected update %s %v", u.System, u.Kind)
	case <-time.After(50 * time.Millisecond):
	}

	// The queue is reused after it ran empty.
	q.put(enc("A", "a4"))
	select {
	case u := <-out:
		if u.Encoder.Name != "a4" {
			t.Errorf("received %+v", u)
		}
	case <-time.After(time.Second):
		t.Fatal("no update after the queue ran empty")
	}
}
