package clickaudit

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/eventqueue"
)

type auditDoer struct {
	mu      sync.Mutex
	status  int
	body    string
	queries []string
	err     error
}

func (d *auditDoer) Do(req *http.Request) (*http.Response, error) {
	if d.err != nil {
		return nil, d.err
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	if err := req.Body.Close(); err != nil {
		return nil, err
	}
	d.mu.Lock()
	d.queries = append(d.queries, string(data))
	d.mu.Unlock()
	status := d.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(d.body)),
	}, nil
}

func (d *auditDoer) Count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.queries)
}

func TestRunOnceReportsDuplicateCountAndCost(t *testing.T) {
	doer := &auditDoer{
		body: `{"duplicate_events":"2","excess_cost":"3.5","conflicting_click_ids":"1"}`,
	}
	a := NewWithDoer(
		"http://clickhouse:8123?database=traffoflex",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		doer,
	)
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	if err := a.RunOnce(context.Background(), "daily", from, to); err != nil {
		t.Fatal(err)
	}
	stats := a.Stats()
	if stats.Daily.DuplicateEvents != 2 || stats.Daily.ExcessCost != 3.5 || stats.Daily.ConflictingClickIDs != 1 || stats.RunTotal != 1 {
		t.Fatalf("unexpected audit stats: %+v", stats)
	}
	if doer.Count() != 1 || !strings.Contains(doer.queries[0], "GROUP BY click_id") || !strings.Contains(doer.queries[0], "created_at < toDateTime('2026-10-02 00:00:00')") {
		t.Fatalf("unexpected audit query: %v", doer.queries)
	}
}

func TestRunOnceRecordsFailure(t *testing.T) {
	doer := &auditDoer{err: errors.New("clickhouse unavailable")}
	a := NewWithDoer(
		"http://clickhouse:8123",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		doer,
	)
	if err := a.RunOnce(
		context.Background(),
		"daily",
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
	); err == nil {
		t.Fatal("expected audit failure")
	}
	stats := a.Stats()
	if stats.FailureTotal != 1 || stats.Daily.Error == "" {
		t.Fatalf("failed audit not visible: %+v", stats)
	}
}

type fakeWAL struct {
	pending atomic.Int64
}

func (w *fakeWAL) RecoveryWindow() (time.Time, time.Time, int64) {
	return time.Time{}, time.Time{}, 1
}

func (w *fakeWAL) Stats() eventqueue.Stats {
	return eventqueue.Stats{RecoveryPending: w.pending.Load() > 0}
}

func TestRecoveryAuditWaitsForWALAndConfirms(t *testing.T) {
	doer := &auditDoer{
		body: `{"duplicate_events":"1","excess_cost":"1.25","conflicting_click_ids":"0"}`,
	}
	a := NewWithDoer(
		"http://clickhouse:8123",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		doer,
	)
	a.grace = time.Millisecond
	a.confirmation = time.Millisecond
	a.retry = time.Millisecond
	a.poll = time.Millisecond
	a.stats.RecoveryPending = true
	wal := &fakeWAL{}
	wal.pending.Store(1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan struct{})
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	go func() {
		a.recovery(ctx, wal, from, from.Add(24*time.Hour))
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	if doer.Count() != 0 {
		t.Fatal("audited before WAL delivery")
	}
	wal.pending.Store(0)
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("recovery audit did not complete")
	}
	stats := a.Stats()
	if doer.Count() != 2 || stats.RecoveryPending || stats.Recovery.DuplicateEvents != 1 {
		t.Fatalf("recovery audit = %+v, queries = %d", stats, doer.Count())
	}
}
