package distribution

import (
	"errors"
	"testing"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestSelectDirectReturnsFirstAvailableConfiguredDestination(t *testing.T) {
	selector := NewSelector()
	destination, err := selector.Select(models.Distribution{
		Mode: models.DistributionDirect,
		Destinations: []models.WeightedTarget{
			{DestinationID: "dst_unhealthy", Weight: 1},
			{DestinationID: "dst_ok", Weight: 1},
		},
	}, []models.Destination{
		destination(
			"dst_unhealthy",
			models.StatusActive,
			models.HealthUnhealthy,
		),
		destination(
			"dst_ok",
			models.StatusActive,
			models.HealthHealthy,
		),
	}, "clk_1")
	if err != nil {
		t.Fatalf(
			"Select returned error: %v",
			err,
		)
	}
	if destination.ID != "dst_ok" {
		t.Fatalf(
			"destination id = %q, want dst_ok",
			destination.ID,
		)
	}
}

func TestSelectWeightedIsDeterministic(t *testing.T) {
	selector := NewSelector()
	distribution := models.Distribution{
		Mode: models.DistributionWeighted,
		Destinations: []models.WeightedTarget{
			{DestinationID: "dst_a", Weight: 50},
			{DestinationID: "dst_b", Weight: 50},
		},
	}
	destinations := []models.Destination{
		destination(
			"dst_a",
			models.StatusActive,
			models.HealthHealthy,
		),
		destination(
			"dst_b",
			models.StatusActive,
			models.HealthHealthy,
		),
	}

	first, err := selector.Select(
		distribution,
		destinations,
		"clk_1",
	)
	if err != nil {
		t.Fatalf(
			"first Select returned error: %v",
			err,
		)
	}
	second, err := selector.Select(
		distribution,
		destinations,
		"clk_1",
	)
	if err != nil {
		t.Fatalf(
			"second Select returned error: %v",
			err,
		)
	}
	if first.ID != second.ID {
		t.Fatalf(
			"weighted selection is not deterministic: %q != %q",
			first.ID,
			second.ID,
		)
	}
}

func TestSelectBestROIPrefersDestinationOrder(t *testing.T) {
	selector := NewSelector()
	selected, err := selector.Select(
		models.Distribution{
			Mode: models.DistributionBestROI,
			Destinations: []models.WeightedTarget{
				{DestinationID: "dst_a", Weight: 1},
				{DestinationID: "dst_b", Weight: 1},
			},
		},
		[]models.Destination{
			destination(
				"dst_b",
				models.StatusActive,
				models.HealthHealthy,
			),
			destination(
				"dst_a",
				models.StatusActive,
				models.HealthHealthy,
			),
		},
		"clk_1",
	)
	if err != nil {
		t.Fatalf(
			"Select returned error: %v",
			err,
		)
	}
	if selected.ID != "dst_b" {
		t.Fatalf(
			"destination = %q, want dst_b",
			selected.ID,
		)
	}
}

func TestSelectReturnsErrorWhenNoDestinationAvailable(t *testing.T) {
	selector := NewSelector()
	_, err := selector.Select(models.Distribution{
		Mode: models.DistributionWeighted,
		Destinations: []models.WeightedTarget{
			{DestinationID: "dst_1", Weight: 100},
		},
	}, []models.Destination{
		destination(
			"dst_1",
			models.StatusPaused,
			models.HealthHealthy,
		),
	}, "clk_1")

	if !errors.Is(
		err,
		ErrNoDestination,
	) {
		t.Fatalf(
			"error = %v, want ErrNoDestination",
			err,
		)
	}
}

func destination(
	id string,
	status models.Status,
	health models.HealthStatus,
) models.Destination {
	return models.Destination{
		ID:           id,
		ManualStatus: status,
		HealthStatus: health,
	}
}
