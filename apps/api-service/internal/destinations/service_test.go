package destinations

import (
	"context"
	"errors"
	"testing"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestServiceCreateValidatesInput(t *testing.T) {
	service := NewService(NewMemoryRepository())

	_, err := service.Create(context.Background(), DestinationRequest{
		Name: "Unsafe",
		URL:  "https://example.com/?redirect_url=https://evil.test",
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

	created, err := service.Create(ctx, DestinationRequest{
		Name: "Destination One",
		URL:  "https://example.com/?subid={click_id}",
	})
	if err != nil {
		t.Fatalf(
			"Create returned error: %v",
			err,
		)
	}
	if created.ID == "" {
		t.Fatal("created destination id is empty")
	}
	if created.ManualStatus != models.StatusActive {
		t.Fatalf(
			"ManualStatus = %q, want active",
			created.ManualStatus,
		)
	}
	if created.HealthStatus != models.HealthUnknown {
		t.Fatalf(
			"HealthStatus = %q, want unknown",
			created.HealthStatus,
		)
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
	if got.Name != "Destination One" {
		t.Fatalf(
			"Name = %q, want Destination One",
			got.Name,
		)
	}

	updated, err := service.Update(ctx, created.ID, DestinationRequest{
		Name:         "Destination Updated",
		URL:          "https://example.org/?subid={click_id}",
		ManualStatus: models.StatusPaused,
		HealthStatus: models.HealthHealthy,
		Redirect:     models.RedirectConfig{Mode: models.RedirectHTTP302},
	})
	if err != nil {
		t.Fatalf(
			"Update returned error: %v",
			err,
		)
	}
	if updated.ManualStatus != models.StatusPaused {
		t.Fatalf(
			"ManualStatus = %q, want paused",
			updated.ManualStatus,
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
