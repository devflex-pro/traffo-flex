package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	apphttp "github.com/devflex/traffoflex/apps/traffic-service/internal/http"
)

type outageLoader struct {
	available atomic.Bool
}

func (l *outageLoader) Load(ctx context.Context) ([]cache.CampaignConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !l.available.Load() {
		return nil, errors.New("mongo unavailable")
	}
	return cache.DemoCampaigns(), nil
}

func TestTrackerRoutesFromSnapshotWithoutMongo(t *testing.T) {
	log := slog.New(slog.NewTextHandler(
		io.Discard,
		nil,
	))
	snapshot := cache.NewSnapshotFile(filepath.Join(
		t.TempDir(),
		"campaigns.json",
	))
	seed := cache.NewPersistentStore(
		cache.StaticLoader{Campaigns: cache.DemoCampaigns()},
		snapshot,
	)
	if err := seed.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	store := cache.NewPersistentStore(
		&outageLoader{},
		snapshot,
	)
	initializeRouting(
		context.Background(),
		store,
		log,
	)
	router := apphttp.NewRouterWithOptions(
		log,
		apphttp.Options{ReadyChecker: store, Cache: store},
	)
	ready := httptest.NewRecorder()
	router.ServeHTTP(
		ready,
		httptest.NewRequest(
			http.MethodGet,
			"/readyz",
			nil,
		),
	)
	if ready.Code != http.StatusOK {
		t.Fatalf(
			"snapshot readiness = %d, want 200",
			ready.Code,
		)
	}
	redirect := httptest.NewRecorder()
	router.ServeHTTP(
		redirect,
		httptest.NewRequest(
			http.MethodGet,
			"/c/demo",
			nil,
		),
	)
	if redirect.Code != http.StatusFound || !strings.HasPrefix(
		redirect.Header().Get("Location"),
		"https://example.com/?subid=clk_",
	) {
		t.Fatalf(
			"snapshot redirect = %d %q",
			redirect.Code,
			redirect.Header().Get("Location"),
		)
	}
}

func TestTrackerWaitsWithoutSnapshotAndRecovers(t *testing.T) {
	log := slog.New(slog.NewTextHandler(
		io.Discard,
		nil,
	))
	loader := &outageLoader{}
	store := cache.NewPersistentStore(
		loader,
		cache.NewSnapshotFile(filepath.Join(
			t.TempDir(),
			"campaigns.json",
		)),
	)
	initializeRouting(
		context.Background(),
		store,
		log,
	)
	router := apphttp.NewRouterWithOptions(
		log,
		apphttp.Options{ReadyChecker: store, Cache: store},
	)
	ready := httptest.NewRecorder()
	router.ServeHTTP(
		ready,
		httptest.NewRequest(
			http.MethodGet,
			"/readyz",
			nil,
		),
	)
	if ready.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"empty routing readiness = %d, want 503",
			ready.Code,
		)
	}
	redirect := httptest.NewRecorder()
	router.ServeHTTP(
		redirect,
		httptest.NewRequest(
			http.MethodGet,
			"/c/demo",
			nil,
		),
	)
	if redirect.Code != http.StatusNotFound {
		t.Fatalf(
			"unknown route = %d, want 404",
			redirect.Code,
		)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		refreshCampaignCache(
			ctx,
			store,
			log,
			10*time.Millisecond,
		)
	}()
	loader.available.Store(true)
	deadline := time.After(time.Second)
	for {
		ready = httptest.NewRecorder()
		router.ServeHTTP(
			ready,
			httptest.NewRequest(
				http.MethodGet,
				"/readyz",
				nil,
			),
		)
		if ready.Code == http.StatusOK {
			break
		}
		select {
		case <-deadline:
			cancel()
			<-done
			t.Fatal("tracker did not become ready after MongoDB recovered")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	<-done
}
