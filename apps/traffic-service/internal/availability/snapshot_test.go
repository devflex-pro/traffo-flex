package availability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestSnapshotKeepsCapsAndROIOutOfRequestPath(t *testing.T) {
	store, capped := testSnapshotStore(t)
	source := &snapshotSource{
		capExceeded: false,
		ranked:      []string{"dst_other", "dst_capped"},
	}
	snapshot := NewSnapshot(store, source, nil, time.Second)
	if _, err := snapshot.Exceeded(context.Background(), capped, time.Now()); !errors.Is(err, ErrCapSnapshotMissing) {
		t.Fatalf("missing snapshot error = %v", err)
	}
	if err := snapshot.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	calls := source.calls
	source.panicOnCall = true
	evaluator := NewEvaluatorWithSources(nil, snapshot, nil, snapshot)
	filtered := evaluator.Filter(
		context.Background(),
		models.Distribution{UniquePolicy: models.UniqueDestinationPolicy{
			SelectionStrategy: models.DistributionBestROI,
			ROIWindowHours:    24,
			MinClicks:         10,
		}},
		[]models.Destination{capped, availableDestination("dst_other")},
		nil,
		"clk_1",
	)
	if len(filtered) != 2 || filtered[0].ID != "dst_other" {
		t.Fatalf("unexpected local ranking: %#v", filtered)
	}
	if source.calls != calls {
		t.Fatalf("request called analytics source: before=%d after=%d", calls, source.calls)
	}
}

func TestSnapshotPreservesLastGoodOnRefreshFailure(t *testing.T) {
	store, capped := testSnapshotStore(t)
	source := &snapshotSource{capExceeded: true, ranked: []string{"dst_other"}}
	snapshot := NewSnapshot(store, source, nil, time.Second)
	if err := snapshot.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	source.fail = true
	if err := snapshot.Refresh(context.Background()); err == nil {
		t.Fatal("expected failed refresh")
	}
	exceeded, err := snapshot.Exceeded(context.Background(), capped, time.Now())
	if err != nil || !exceeded {
		t.Fatalf("last good cap lost: exceeded=%t err=%v", exceeded, err)
	}
	ranked, err := snapshot.RankByROI(
		context.Background(),
		[]string{"dst_capped", "dst_other"},
		24,
		10,
		time.Now(),
	)
	if err != nil || len(ranked) != 1 || ranked[0] != "dst_other" {
		t.Fatalf("last good ROI lost: ranked=%#v err=%v", ranked, err)
	}
	if snapshot.Stats().LastError == "" || snapshot.Stats().RefreshFailures != 1 {
		t.Fatalf("refresh failure not visible: %#v", snapshot.Stats())
	}
}

func TestMissingCapSnapshotExcludesDestination(t *testing.T) {
	store, capped := testSnapshotStore(t)
	snapshot := NewSnapshot(store, &snapshotSource{fail: true}, nil, time.Second)
	evaluator := NewEvaluatorWithSources(nil, snapshot, nil, snapshot)
	filtered := evaluator.Filter(
		context.Background(),
		models.Distribution{},
		[]models.Destination{capped, availableDestination("dst_other")},
		nil,
		"clk_1",
	)
	if len(filtered) != 1 || filtered[0].ID != "dst_other" {
		t.Fatalf("missing cap snapshot did not exclude capped destination: %#v", filtered)
	}
	if snapshot.Stats().MissingCapChecks != 1 {
		t.Fatalf("missing cap count = %d", snapshot.Stats().MissingCapChecks)
	}
}

func TestMissingROISnapshotUsesFallbackOrder(t *testing.T) {
	store, _ := testSnapshotStore(t)
	snapshot := NewSnapshot(store, &snapshotSource{fail: true}, nil, time.Second)
	evaluator := NewEvaluatorWithSources(nil, nil, nil, snapshot)
	filtered := evaluator.Filter(
		context.Background(),
		models.Distribution{UniquePolicy: models.UniqueDestinationPolicy{
			SelectionStrategy: models.DistributionBestROI,
			FallbackStrategy:  models.DistributionWaterfall,
			ROIWindowHours:    24,
			MinClicks:         10,
		}},
		[]models.Destination{availableDestination("dst_first"), availableDestination("dst_second")},
		nil,
		"clk_1",
	)
	if len(filtered) != 2 || filtered[0].ID != "dst_first" {
		t.Fatalf("unexpected fallback order: %#v", filtered)
	}
}

func testSnapshotStore(t *testing.T) (*cache.Store, models.Destination) {
	t.Helper()
	capped := availableDestination("dst_capped")
	capped.Caps = models.DestinationCaps{
		Enabled: true,
		Rules: []models.DestinationCapRule{{
			Metric:      models.CapMetricClicks,
			WindowHours: 24,
			Limit:       100,
		}},
	}
	store := cache.NewStore(cache.StaticLoader{Campaigns: []cache.CampaignConfig{{
		Campaign: models.Campaign{ID: "cmp_1", Slug: "test", Status: models.StatusActive},
		Streams: []models.Stream{{
			Status: models.StatusActive,
			Distribution: models.Distribution{
				Destinations: []models.WeightedTarget{
					{DestinationID: "dst_capped", Weight: 1},
					{DestinationID: "dst_other", Weight: 1},
				},
				UniquePolicy: models.UniqueDestinationPolicy{
					SelectionStrategy: models.DistributionBestROI,
					ROIWindowHours:    24,
					MinClicks:         10,
				},
			},
		}},
		Destinations: []models.Destination{capped, availableDestination("dst_other")},
	}}})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	return store, capped
}

type snapshotSource struct {
	capExceeded bool
	ranked      []string
	fail        bool
	panicOnCall bool
	calls       int
}

func (s *snapshotSource) Exceeded(
	ctx context.Context,
	destination models.Destination,
	now time.Time,
) (bool, error) {
	s.calls++
	if s.panicOnCall {
		panic("analytics source called on request path")
	}
	if s.fail {
		return false, errors.New("clickhouse unavailable")
	}
	return s.capExceeded, ctx.Err()
}

func (s *snapshotSource) RankByROI(
	ctx context.Context,
	destinationIDs []string,
	windowHours int,
	minClicks int,
	now time.Time,
) ([]string, error) {
	s.calls++
	if s.panicOnCall {
		panic("analytics source called on request path")
	}
	if s.fail {
		return nil, errors.New("clickhouse unavailable")
	}
	return s.ranked, ctx.Err()
}
