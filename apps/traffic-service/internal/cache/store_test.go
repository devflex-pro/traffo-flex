package cache

import (
	"context"
	"errors"
	"testing"
)

func TestStoreReloadAndGetBySlug(t *testing.T) {
	store := NewStore(StaticLoader{Campaigns: DemoCampaigns()})

	if err := store.Reload(context.Background()); err != nil {
		t.Fatalf(
			"Reload returned error: %v",
			err,
		)
	}

	campaign, err := store.GetBySlug("demo")
	if err != nil {
		t.Fatalf(
			"GetBySlug returned error: %v",
			err,
		)
	}
	if campaign.Campaign.ID != "cmp_demo" {
		t.Fatalf(
			"campaign id = %q, want cmp_demo",
			campaign.Campaign.ID,
		)
	}
	byPublicID, err := store.GetByPublicID("demo-public")
	if err != nil {
		t.Fatalf(
			"GetByPublicID returned error: %v",
			err,
		)
	}
	if byPublicID.Campaign.ID != "cmp_demo" {
		t.Fatalf(
			"public id campaign id = %q, want cmp_demo",
			byPublicID.Campaign.ID,
		)
	}
	byToken, err := store.GetByToken("demo-token")
	if err != nil {
		t.Fatalf(
			"GetByToken returned error: %v",
			err,
		)
	}
	if byToken.Campaign.ID != "cmp_demo" {
		t.Fatalf(
			"token campaign id = %q, want cmp_demo",
			byToken.Campaign.ID,
		)
	}
	if store.ReloadedAt().IsZero() {
		t.Fatal("ReloadedAt is zero")
	}
}

func TestStoreGetBySlugNotFound(t *testing.T) {
	store := NewStore(StaticLoader{Campaigns: DemoCampaigns()})
	if err := store.Reload(context.Background()); err != nil {
		t.Fatalf(
			"Reload returned error: %v",
			err,
		)
	}

	if _, err := store.GetBySlug("missing"); !errors.Is(
		err,
		ErrCampaignNotFound,
	) {
		t.Fatalf(
			"error = %v, want ErrCampaignNotFound",
			err,
		)
	}
}
