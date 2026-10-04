package eventqueue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestQueueDoesNotWaitForDeliveryAndCountsOverflow(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	q := New[int](
		"test",
		1,
		nil,
		func(ctx context.Context, value int) error {
			if value == 1 {
				close(started)
				select {
				case <-release:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		},
	)
	if err := q.Submit(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := q.Submit(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	if err := q.Submit(context.Background(), 3); !errors.Is(err, ErrFull) {
		t.Fatalf("submit to full queue = %v, want ErrFull", err)
	}
	if time.Since(before) > 100*time.Millisecond {
		t.Fatal("submit waited for event delivery")
	}
	close(release)
	if err := q.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	stats := q.Stats()
	if stats.AcceptedTotal != 2 || stats.DeliveredTotal != 2 ||
		stats.OverflowTotal != 1 || stats.Pending != 0 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	if err := q.Submit(context.Background(), 4); !errors.Is(err, ErrClosed) {
		t.Fatalf("submit after close = %v, want ErrClosed", err)
	}
}

func TestQueueRecordsDeliveryFailure(t *testing.T) {
	q := New[int](
		"test",
		1,
		nil,
		func(context.Context, int) error {
			return errors.New("broker unavailable")
		},
	)
	if err := q.Submit(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := q.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	stats := q.Stats()
	if stats.FailedTotal != 1 || stats.DeliveredTotal != 0 || stats.Pending != 0 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestAcceptedEventOutlivesRequestContext(t *testing.T) {
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	delivered := make(chan struct{})
	q := New[int](
		"test",
		1,
		nil,
		func(ctx context.Context, value int) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			close(delivered)
			return nil
		},
	)
	if err := q.Submit(requestCtx, 1); err != nil {
		t.Fatal(err)
	}
	cancelRequest()
	if err := q.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-delivered:
	default:
		t.Fatal("accepted event was canceled with the request")
	}
}

func TestQueueBatchesReadyEvents(t *testing.T) {
	var batches [][]int
	q := NewBatched[int](
		"test",
		4,
		3,
		20*time.Millisecond,
		nil,
		func(ctx context.Context, events []int) error {
			batches = append(batches, append([]int(nil), events...))
			return ctx.Err()
		},
	)
	for _, value := range []int{1, 2, 3} {
		if err := q.Submit(context.Background(), value); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0]) != 3 ||
		batches[0][0] != 1 || batches[0][2] != 3 {
		t.Fatalf("unexpected batches: %#v", batches)
	}
}

func TestQueueCloseDeadlineCancelsDelivery(t *testing.T) {
	started := make(chan struct{})
	q := New[int](
		"test",
		1,
		nil,
		func(ctx context.Context, value int) error {
			if value == 1 {
				close(started)
			}
			<-ctx.Done()
			return ctx.Err()
		},
	)
	if err := q.Submit(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := q.Submit(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := q.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("close = %v, want deadline exceeded", err)
	}
	<-q.Done()
	stats := q.Stats()
	if stats.FailedTotal != 1 || stats.StoppedTotal != 1 || stats.Pending != 0 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestConcurrentSubmitAndClose(t *testing.T) {
	q := New[int](
		"test",
		32,
		nil,
		func(context.Context, int) error { return nil },
	)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(value int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				err := q.Submit(context.Background(), value)
				if err != nil && !errors.Is(err, ErrFull) && !errors.Is(err, ErrClosed) {
					t.Errorf("submit error: %v", err)
				}
			}
		}(i)
	}
	if err := q.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if q.Stats().Pending != 0 {
		t.Fatalf("pending events after close: %#v", q.Stats())
	}
}
