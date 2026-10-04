package healthcheck

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestWorkerBoundsConcurrentProbes(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		current := active.Add(1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	campaigns := cache.DemoCampaigns()
	campaigns[0].Destinations = nil
	for index := 0; index < 6; index++ {
		campaigns[0].Destinations = append(campaigns[0].Destinations, models.Destination{
			ID:           string(rune('a' + index)),
			URL:          "https://destination.example/health",
			ManualStatus: models.StatusActive,
			HealthStatus: models.HealthUnknown,
		})
	}
	store := cache.NewStore(cache.StaticLoader{Campaigns: campaigns})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithClient(store, nil, client)
	worker := NewWorker(slog.New(slog.NewTextHandler(io.Discard, nil)), service, time.Minute, 2)
	worker.runOnce(context.Background())
	if maximum.Load() != 2 {
		t.Fatalf("maximum concurrent probes = %d, want 2", maximum.Load())
	}
}
