package antirepeat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func testManager(t *testing.T) (*Manager, StreamKey, models.UniqueDestinationPolicy) {
	t.Helper()
	m, err := NewManager(Config{
		ExpectedKeysPerBucket: 100,
		FalsePositiveRate:     0.000001,
		MaxBytes:              1024 * 1024,
		BucketCount:           4,
		SnapshotPath:          filepath.Join(t.TempDir(), "snapshot.gob"),
		SnapshotInterval:      time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	key := StreamKey{CampaignID: "campaign", StreamID: "stream"}
	policy := models.UniqueDestinationPolicy{
		Enabled:            true,
		UserKey:            "source_click_id",
		HistoryWindowHours: 24,
		ExhaustedMode:      models.UniqueExhaustedNoDestination,
	}
	m.Configure([]cache.CampaignConfig{{
		Campaign: models.Campaign{ID: key.CampaignID, Status: models.StatusActive},
		Streams:  []models.Stream{{ID: key.StreamID, Status: models.StatusActive, Distribution: models.Distribution{UniquePolicy: policy}}},
	}})
	return m, key, policy
}

func first(destinations []models.Destination) (models.Destination, error) {
	if len(destinations) == 0 {
		return models.Destination{}, errors.New("no destination")
	}
	return destinations[0], nil
}

func TestConcurrentSelectionNeverRepeats(t *testing.T) {
	m, key, policy := testManager(t)
	if !m.SetReady(key, policy.UserKey, policy.HistoryWindowHours) {
		t.Fatal("failed to mark stream ready")
	}
	destinations := []models.Destination{{ID: "a"}, {ID: "b"}}
	selected := make(chan string, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := m.SelectAndMark(key, policy, "user", destinations, time.Now().UTC(), first)
			if err != nil {
				selected <- err.Error()
				return
			}
			selected <- result.ID
		}()
	}
	wg.Wait()
	close(selected)
	seen := map[string]bool{}
	for id := range selected {
		seen[id] = true
	}
	if !seen["a"] || !seen["b"] || len(seen) != 2 {
		t.Fatalf("expected distinct destinations, got %v", seen)
	}
	_, err := m.SelectAndMark(key, policy, "user", destinations, time.Now().UTC(), first)
	if !errors.Is(err, ErrHistoryExhausted) {
		t.Fatalf("expected exhausted, got %v", err)
	}
}

func TestSnapshotRequiresBackfillBeforeReady(t *testing.T) {
	m, key, policy := testManager(t)
	m.SetReady(key, policy.UserKey, policy.HistoryWindowHours)
	_, err := m.SelectAndMark(key, policy, "user", []models.Destination{{ID: "a"}}, time.Now().UTC(), first)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	restored, err := NewManager(m.config)
	if err != nil {
		t.Fatal(err)
	}
	restored.Configure([]cache.CampaignConfig{{
		Campaign: models.Campaign{ID: key.CampaignID, Status: models.StatusActive},
		Streams:  []models.Stream{{ID: key.StreamID, Status: models.StatusActive, Distribution: models.Distribution{UniquePolicy: policy}}},
	}})
	if err := restored.Load(); err != nil {
		t.Fatal(err)
	}
	_, err = restored.SelectAndMark(key, policy, "user", []models.Destination{{ID: "a"}}, time.Now().UTC(), first)
	if !errors.Is(err, ErrHistoryUnavailable) {
		t.Fatalf("expected history unavailable before backfill, got %v", err)
	}
	restored.SetReady(key, policy.UserKey, policy.HistoryWindowHours)
	_, err = restored.SelectAndMark(key, policy, "user", []models.Destination{{ID: "a"}}, time.Now().UTC(), first)
	if !errors.Is(err, ErrHistoryExhausted) {
		t.Fatalf("expected restored history, got %v", err)
	}
}

func TestBackfillSeedsHistory(t *testing.T) {
	m, key, policy := testManager(t)
	source := NewBackfill("http://clickhouse:8123")
	source.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"event_time":` + strconv.FormatInt(time.Now().UTC().Unix(), 10) + `,"destination_id":"a","user_value":"user"}` + "\n")),
		}, nil
	})}
	if err := source.Seed(context.Background(), PendingStream{Key: key, UserKey: policy.UserKey, WindowHours: policy.HistoryWindowHours}, m); err != nil {
		t.Fatal(err)
	}
	m.SetReady(key, policy.UserKey, policy.HistoryWindowHours)
	selected, err := m.SelectAndMark(key, policy, "user", []models.Destination{{ID: "a"}, {ID: "b"}}, time.Now().UTC(), first)
	if err != nil || selected.ID != "b" {
		t.Fatalf("expected b after backfill, got %q, %v", selected.ID, err)
	}
}

func TestFailedBackfillLeavesStreamPending(t *testing.T) {
	m, key, policy := testManager(t)
	source := NewBackfill("http://clickhouse:8123")
	source.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader("database unavailable")),
		}, nil
	})}
	if err := source.Seed(context.Background(), PendingStream{Key: key, UserKey: policy.UserKey, WindowHours: policy.HistoryWindowHours}, m); err == nil {
		t.Fatal("expected backfill error")
	}
	if got := m.Stats().PendingStreams; got != 1 {
		t.Fatalf("expected stream to remain pending, got %d", got)
	}
}

func TestHistoryExpiresAfterWindow(t *testing.T) {
	m, key, policy := testManager(t)
	if !m.SetReady(key, policy.UserKey, policy.HistoryWindowHours) {
		t.Fatal("failed to mark stream ready")
	}
	start := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	destinations := []models.Destination{{ID: "a"}}
	if _, err := m.SelectAndMark(key, policy, "user", destinations, start, first); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SelectAndMark(key, policy, "user", destinations, start.Add(time.Hour), first); !errors.Is(err, ErrHistoryExhausted) {
		t.Fatalf("expected recent history to block repeat, got %v", err)
	}
	if _, err := m.SelectAndMark(key, policy, "user", destinations, start.Add(30*time.Hour), first); err != nil {
		t.Fatalf("expected old history to expire, got %v", err)
	}
}

func TestSaturatedBucketRoutesToFallback(t *testing.T) {
	m, key, policy := testManager(t)
	m.config.ExpectedKeysPerBucket = 1
	if !m.SetReady(key, policy.UserKey, policy.HistoryWindowHours) {
		t.Fatal("failed to mark stream ready")
	}
	destinations := []models.Destination{{ID: "a"}}
	if _, err := m.SelectAndMark(key, policy, "first", destinations, time.Now().UTC(), first); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SelectAndMark(key, policy, "second", destinations, time.Now().UTC(), first); !errors.Is(err, ErrHistoryUnavailable) {
		t.Fatalf("expected saturated filter to require fallback, got %v", err)
	}
	if m.Stats().SaturatedTotal != 1 {
		t.Fatalf("expected saturation metric, got %+v", m.Stats())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
