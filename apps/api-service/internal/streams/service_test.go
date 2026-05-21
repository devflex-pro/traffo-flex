package streams

import (
	"context"
	"errors"
	"testing"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestServiceCreateValidatesInput(t *testing.T) {
	service := NewService(NewMemoryRepository())

	_, err := service.Create(context.Background(), "cmp_1", StreamRequest{
		Name:     "Invalid",
		Priority: -1,
		Distribution: models.Distribution{
			Mode: models.DistributionWeighted,
			Destinations: []models.WeightedTarget{
				{DestinationID: "dst_1", Weight: 10},
			},
		},
	})
	if !errors.Is(
		err,
		ErrInvalidInput,
	) {
		t.Fatalf(
			"error = %v, want ErrInvalidInput",
			err,
		)
	}
}

func TestServiceCRUD(t *testing.T) {
	service := NewService(NewMemoryRepository())
	ctx := context.Background()

	req := StreamRequest{
		Name:     "Primary Stream",
		Priority: 10,
		Conditions: []models.Condition{
			{Field: "geo_country", Operator: models.OperatorEQ, Value: "US"},
		},
		Distribution: models.Distribution{
			Mode: models.DistributionWeighted,
			Destinations: []models.WeightedTarget{
				{DestinationID: "dst_1", Weight: 100},
			},
		},
	}

	created, err := service.Create(
		ctx,
		"cmp_1",
		req,
	)
	if err != nil {
		t.Fatalf(
			"Create returned error: %v",
			err,
		)
	}
	if created.ID == "" {
		t.Fatal("created stream id is empty")
	}

	streams, err := service.ListByCampaign(
		ctx,
		"cmp_1",
	)
	if err != nil {
		t.Fatalf(
			"ListByCampaign returned error: %v",
			err,
		)
	}
	if len(streams) != 1 {
		t.Fatalf(
			"len(streams) = %d, want 1",
			len(streams),
		)
	}

	req.Name = "Updated Stream"
	req.Priority = 20
	updated, err := service.Update(
		ctx,
		created.ID,
		req,
	)
	if err != nil {
		t.Fatalf(
			"Update returned error: %v",
			err,
		)
	}
	if updated.Name != "Updated Stream" {
		t.Fatalf(
			"Name = %q, want Updated Stream",
			updated.Name,
		)
	}

	if err := service.Delete(
		ctx,
		created.ID,
	); err != nil {
		t.Fatalf(
			"Delete returned error: %v",
			err,
		)
	}
	if err := service.Delete(
		ctx,
		created.ID,
	); !errors.Is(
		err,
		ErrNotFound,
	) {
		t.Fatalf(
			"second Delete error = %v, want ErrNotFound",
			err,
		)
	}
}
