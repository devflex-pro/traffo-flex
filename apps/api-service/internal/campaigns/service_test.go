package campaigns

import (
	"context"
	"errors"
	"testing"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestServiceCreateValidatesInput(t *testing.T) {
	service := NewService(NewMemoryRepository())

	_, err := service.Create(context.Background(), CampaignRequest{
		Name:   "Invalid",
		Slug:   "Invalid Slug",
		Status: models.StatusActive,
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

	created, err := service.Create(ctx, CampaignRequest{
		Name:   "Campaign One",
		Slug:   "campaign-one",
		Status: models.StatusActive,
	})
	if err != nil {
		t.Fatalf(
			"Create returned error: %v",
			err,
		)
	}
	if created.ID == "" {
		t.Fatal("created campaign id is empty")
	}

	got, err := service.Get(
		ctx,
		created.ID,
	)
	if err != nil {
		t.Fatalf(
			"Get returned error: %v",
			err,
		)
	}
	if got.Name != "Campaign One" {
		t.Fatalf(
			"Name = %q, want Campaign One",
			got.Name,
		)
	}

	updated, err := service.Update(ctx, created.ID, CampaignRequest{
		Name:   "Campaign Updated",
		Slug:   "campaign-updated",
		Status: models.StatusPaused,
	})
	if err != nil {
		t.Fatalf(
			"Update returned error: %v",
			err,
		)
	}
	if updated.Status != models.StatusPaused {
		t.Fatalf(
			"Status = %q, want paused",
			updated.Status,
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
	if _, err := service.Get(
		ctx,
		created.ID,
	); !errors.Is(
		err,
		ErrNotFound,
	) {
		t.Fatalf(
			"Get after delete error = %v, want ErrNotFound",
			err,
		)
	}
}
