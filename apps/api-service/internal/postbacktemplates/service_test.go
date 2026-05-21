package postbacktemplates

import (
	"context"
	"errors"
	"testing"
)

func TestServiceCreateValidatesInput(t *testing.T) {
	service := NewService(NewMemoryRepository())

	_, err := service.Create(context.Background(), Request{
		NetworkID: "net_1",
		Name:      "Invalid",
		Slug:      "invalid-template",
		Mapping:   map[string]string{"ClickID": "cid"},
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

	created, err := service.Create(ctx, Request{
		NetworkID: "net_1",
		Name:      "Default Template",
		Slug:      "default-template",
		Mapping:   map[string]string{"click_id": "cid", "transaction_id": "tx"},
	})
	if err != nil {
		t.Fatalf(
			"Create returned error: %v",
			err,
		)
	}
	if created.ID == "" {
		t.Fatal("created template id is empty")
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
	if got.Mapping["click_id"] != "cid" {
		t.Fatalf(
			"click_id mapping = %q, want cid",
			got.Mapping["click_id"],
		)
	}

	updated, err := service.Update(ctx, created.ID, Request{
		NetworkID: "net_1",
		Name:      "Updated Template",
		Slug:      "updated-template",
		Mapping:   map[string]string{"click_id": "subid"},
	})
	if err != nil {
		t.Fatalf(
			"Update returned error: %v",
			err,
		)
	}
	if updated.Mapping["click_id"] != "subid" {
		t.Fatalf(
			"click_id mapping = %q, want subid",
			updated.Mapping["click_id"],
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
