package eventqueue

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestClickWALReplaysAfterRestartAndRetries(t *testing.T) {
	dir := t.TempDir()
	cfg := testWALConfig(dir)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	first, err := OpenClickWAL(
		cfg,
		logger,
		func(ctx context.Context, events []models.ClickEvent) error {
			return errors.New("broker unavailable")
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 10, 2, 23, 59, 0, 0, time.UTC)
	for i, id := range []string{"one", "two"} {
		if err := first.Log(context.Background(), models.ClickEvent{
			ClickID:   id,
			CreatedAt: createdAt.Add(time.Duration(i) * 2 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	awaitWAL(t, func() bool { return first.Stats().RetryTotal > 0 })
	if err := first.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var delivered []string
	second, err := OpenClickWAL(
		cfg,
		logger,
		func(ctx context.Context, events []models.ClickEvent) error {
			mu.Lock()
			defer mu.Unlock()
			for _, event := range events {
				delivered = append(delivered, event.ClickID)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	from, to, recovered := second.RecoveryWindow()
	if recovered != 2 || !from.Equal(createdAt) || !to.Equal(createdAt.Add(2*time.Minute)) {
		t.Fatalf("recovery window = %s..%s count %d", from, to, recovered)
	}
	awaitWAL(t, func() bool { return second.Stats().Pending == 0 })
	if second.Stats().RecoveryPending {
		t.Fatal("recovery should be marked complete after acknowledgement")
	}
	if err := second.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(delivered) != 2 || delivered[0] != "one" || delivered[1] != "two" {
		t.Fatalf("replayed clicks = %v", delivered)
	}
	third, err := OpenClickWAL(
		cfg,
		logger,
		func(ctx context.Context, events []models.ClickEvent) error {
			t.Error("acknowledged click replayed")
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if third.Stats().Pending != 0 {
		t.Fatalf("pending after checkpoint = %d", third.Stats().Pending)
	}
	if err := third.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestClickWALBoundsDiskAndRepairsPartialTail(t *testing.T) {
	dir := t.TempDir()
	event := models.ClickEvent{ClickID: "one"}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	cfg := testWALConfig(dir)
	cfg.MaxBytes = int64(len(data) + 8)
	cfg.SegmentBytes = cfg.MaxBytes
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	w, err := OpenClickWAL(
		cfg,
		logger,
		func(ctx context.Context, events []models.ClickEvent) error {
			<-ctx.Done()
			return ctx.Err()
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Log(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := w.Log(context.Background(), event); !errors.Is(err, ErrWALFull) {
		t.Fatalf("second append error = %v", err)
	}
	if got := w.Stats(); got.DiskBytes > got.MaxDiskBytes || got.OverflowTotal != 1 || got.Pending != 1 {
		t.Fatalf("unexpected WAL stats: %+v", got)
	}
	if err := w.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "00000000000000000002.wal")
	if err := os.WriteFile(path, []byte{0, 0, 0}, 0600); err != nil {
		t.Fatal(err)
	}
	// The previous segment is intact; only the final incomplete record is cut.
	var seen []string
	w, err = OpenClickWAL(
		cfg,
		logger,
		func(ctx context.Context, events []models.ClickEvent) error {
			for _, event := range events {
				seen = append(seen, event.ClickID)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	awaitWAL(t, func() bool { return w.Stats().Pending == 0 })
	if err := w.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != "one" {
		t.Fatalf("recovered clicks = %v", seen)
	}
}

func TestClickWALReclaimsAcknowledgedCapacity(t *testing.T) {
	event := models.ClickEvent{ClickID: "one"}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	cfg := testWALConfig(t.TempDir())
	cfg.MaxBytes = int64(len(data) + 8)
	cfg.SegmentBytes = cfg.MaxBytes
	w, err := OpenClickWAL(
		cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(ctx context.Context, events []models.ClickEvent) error {
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := w.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	if err := w.Log(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	awaitWAL(t, func() bool { return w.Stats().Pending == 0 })
	if err := w.Log(context.Background(), event); err != nil {
		t.Fatalf("append after acknowledgement: %v", err)
	}
	awaitWAL(t, func() bool { return w.Stats().DeliveredTotal == 2 })
	if got := w.Stats(); got.DiskBytes > got.MaxDiskBytes || got.OverflowTotal != 0 {
		t.Fatalf("unexpected WAL stats: %+v", got)
	}
}

func TestClickWALConcurrentAppend(t *testing.T) {
	cfg := testWALConfig(t.TempDir())
	cfg.MaxBytes = 4 * 1024 * 1024
	cfg.SegmentBytes = 16 * 1024
	w, err := OpenClickWAL(
		cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(ctx context.Context, events []models.ClickEvent) error {
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 1000)
	for worker := 0; worker < 10; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				id := strconv.Itoa(worker*100 + i)
				if err := w.Log(context.Background(), models.ClickEvent{ClickID: id}); err != nil {
					errs <- err
				}
			}
		}(worker)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	awaitWAL(t, func() bool { return w.Stats().DeliveredTotal == 1000 })
	if err := w.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := w.Stats(); got.Pending != 0 || got.OverflowTotal != 0 || got.DiskBytes > got.MaxDiskBytes {
		t.Fatalf("unexpected WAL stats: %+v", got)
	}
}

func testWALConfig(dir string) ClickWALConfig {
	return ClickWALConfig{
		Dir:          dir,
		MaxBytes:     1024 * 1024,
		SegmentBytes: 1024,
		BatchSize:    10,
		RetryDelay:   time.Millisecond,
	}
}

func awaitWAL(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for click WAL")
}
