package conversions

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

type testAttributionLookup struct {
	found bool
	err   error
}

func (l *testAttributionLookup) Find(
	ctx context.Context,
	ownerID string,
	clickID string,
) (ClickAttribution, bool, error) {
	return ClickAttribution{CampaignID: "cmp_1"}, l.found, l.err
}

type testAttributionSink struct {
	err     error
	events  []string
	batches int
}

func (s *testAttributionSink) WriteAttributions(
	ctx context.Context,
	events []AttributionEvent,
) error {
	if s.err != nil {
		return s.err
	}
	s.batches++
	for _, event := range events {
		s.events = append(s.events, event.Conversion.ConversionID)
	}
	return nil
}

func TestAttributionWorkerRetriesLateClickAndFailedPublish(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	event, created, err := NewService(repo).Process(ctx, testConversion())
	if err != nil || !created {
		t.Fatalf("create conversion: %v, %v", created, err)
	}
	lookup := &testAttributionLookup{}
	sink := &testAttributionSink{}
	worker := NewAttributionWorker(
		repo,
		lookup,
		sink,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		time.Second,
	)
	worker.RunOnce(ctx)
	pending, err := repo.PendingAttribution(ctx, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("retry should be delayed: %v, %v", pending, err)
	}
	if err := repo.DeferAttribution(ctx, event.ConversionID, 1, time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	lookup.found = true
	sink.err = errors.New("broker unavailable")
	worker.RunOnce(ctx)
	if len(sink.events) != 0 {
		t.Fatal("failed publish was recorded")
	}
	if err := repo.DeferAttribution(ctx, event.ConversionID, 2, time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	sink.err = nil
	worker.RunOnce(ctx)
	pending, err = repo.PendingAttribution(ctx, 10)
	if err != nil || len(pending) != 0 || len(sink.events) != 1 {
		t.Fatalf("attribution state: %v, %v, %v", pending, err, sink.events)
	}
}

func TestClickHouseAttributionLookupEscapesClickID(t *testing.T) {
	lookup := NewClickHouseAttributionLookup("http://clickhouse:8123")
	lookup.client.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		if !strings.Contains(string(body), `click_id = 'abc\' OR 1=1 --'`) {
			t.Errorf("unsafe query: %s", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"campaign_id":"cmp_1"}`)),
		}, nil
	})
	click, found, err := lookup.Find(
		context.Background(),
		"",
		"abc' OR 1=1 --",
	)
	if err != nil || !found || click.CampaignID != "cmp_1" {
		t.Fatalf("lookup: %+v, %v, %v", click, found, err)
	}
}

func TestAttributionWorkerPublishesBatch(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo)
	for _, transactionID := range []string{"tx_1", "tx_2"} {
		conversion := testConversion()
		conversion.TransactionID = transactionID
		if _, _, err := service.Process(ctx, conversion); err != nil {
			t.Fatal(err)
		}
	}
	sink := &testAttributionSink{}
	worker := NewAttributionWorker(
		repo,
		&testAttributionLookup{found: true},
		sink,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		time.Second,
	)
	worker.RunOnce(ctx)
	if sink.batches != 1 || len(sink.events) != 2 {
		t.Fatalf("batches = %d, events = %d", sink.batches, len(sink.events))
	}
	pending, err := repo.PendingAttribution(ctx, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending = %v, %v", pending, err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
