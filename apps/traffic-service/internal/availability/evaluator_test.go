package availability

import (
	"context"
	"testing"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestEvaluatorFiltersScheduleAndCaps(t *testing.T) {
	now := time.Date(
		2026,
		5,
		18,
		10,
		0,
		0,
		0,
		time.UTC,
	)
	evaluator := NewEvaluatorWithClock(
		nil,
		staticCapChecker{exceeded: map[string]bool{"dst_capped": true}},
		func() time.Time {
			return now
		},
	)
	destinations := []models.Destination{
		availableDestination("dst_ok"),
		{
			ID:           "dst_scheduled_out",
			ManualStatus: models.StatusActive,
			HealthStatus: models.HealthHealthy,
			Schedule: models.DestinationSchedule{
				Enabled:  true,
				Timezone: "UTC",
				Windows: []models.ScheduleWindow{
					{
						Weekdays:  []models.Weekday{models.WeekdayTuesday},
						StartTime: "09:00",
						EndTime:   "18:00",
					},
				},
			},
		},
		{
			ID:           "dst_capped",
			ManualStatus: models.StatusActive,
			HealthStatus: models.HealthHealthy,
			Caps: models.DestinationCaps{
				Enabled: true,
				Rules: []models.DestinationCapRule{
					{
						Metric:      models.CapMetricClicks,
						WindowHours: 24,
						Limit:       100,
					},
				},
			},
		},
	}

	filtered := evaluator.Filter(
		context.Background(),
		models.Distribution{},
		destinations,
		nil,
		"clk_1",
	)
	if len(filtered) != 1 {
		t.Fatalf(
			"filtered len = %d, want 1",
			len(filtered),
		)
	}
	if filtered[0].ID != "dst_ok" {
		t.Fatalf(
			"destination = %q, want dst_ok",
			filtered[0].ID,
		)
	}
}

func TestEvaluatorFiltersUsedDestinations(t *testing.T) {
	evaluator := NewEvaluatorWithClock(
		nil,
		staticPolicyStore{used: map[string]struct{}{"dst_seen": {}}},
		func() time.Time {
			return time.Date(
				2026,
				5,
				18,
				10,
				0,
				0,
				0,
				time.UTC,
			)
		},
	)
	filtered := evaluator.Filter(
		context.Background(),
		models.Distribution{
			UniquePolicy: models.UniqueDestinationPolicy{
				Enabled:            true,
				UserKey:            "source_click_id",
				HistoryWindowHours: 168,
				ExhaustedMode:      models.UniqueExhaustedAllowRepeat,
				SelectionStrategy:  models.DistributionWaterfall,
			},
		},
		[]models.Destination{
			availableDestination("dst_seen"),
			availableDestination("dst_new"),
		},
		map[string]string{"source_click_id": "user_1"},
		"clk_1",
	)
	if len(filtered) != 1 || filtered[0].ID != "dst_new" {
		t.Fatalf(
			"filtered = %#v, want dst_new only",
			filtered,
		)
	}
}

func availableDestination(id string) models.Destination {
	return models.Destination{
		ID:           id,
		ManualStatus: models.StatusActive,
		HealthStatus: models.HealthHealthy,
	}
}

type staticCapChecker struct {
	exceeded map[string]bool
}

func (c staticCapChecker) Exceeded(
	ctx context.Context,
	destination models.Destination,
	now time.Time,
) (
	bool,
	error,
) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return c.exceeded[destination.ID], nil
}

type staticPolicyStore struct {
	used map[string]struct{}
}

func (s staticPolicyStore) Exceeded(
	ctx context.Context,
	destination models.Destination,
	now time.Time,
) (
	bool,
	error,
) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func (s staticPolicyStore) UsedDestinations(
	ctx context.Context,
	userKey string,
	userValue string,
	windowHours int,
	now time.Time,
) (
	map[string]struct{},
	error,
) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.used, nil
}
