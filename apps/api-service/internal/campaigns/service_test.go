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

func TestCampaignPricingDefaultsAndUpdates(t *testing.T) {
	service := NewService(NewMemoryRepository())
	ctx := context.Background()
	req := CampaignRequest{
		Name:   "Pricing campaign",
		Slug:   "pricing-campaign",
		Status: models.StatusPaused,
	}
	campaign, err := service.Create(
		ctx,
		req,
	)
	if err != nil {
		t.Fatal(err)
	}
	if campaign.PricingModel != models.PricingCPC {
		t.Fatalf(
			"default pricing = %q, want cpc",
			campaign.PricingModel,
		)
	}
	req.PricingModel = models.PricingCPM
	campaign, err = service.Update(
		ctx,
		campaign.ID,
		req,
	)
	if err != nil {
		t.Fatal(err)
	}
	req.PricingModel = ""
	req.Status = models.StatusArchived
	campaign, err = service.Update(
		ctx,
		campaign.ID,
		req,
	)
	if err != nil {
		t.Fatal(err)
	}
	if campaign.PricingModel != models.PricingCPM {
		t.Fatal("omitted pricing model reset CPM on status change")
	}
	campaign, err = service.UpdateTrackingParams(
		ctx,
		campaign.ID,
		[]models.TrackingParam{{Key: "cost", Value: "[CPV_PRICE]"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if campaign.PricingModel != models.PricingCPM {
		t.Fatal("tracking URL update reset CPM")
	}
	req.PricingModel = models.PricingCPC
	campaign, err = service.Update(
		ctx,
		campaign.ID,
		req,
	)
	if err != nil {
		t.Fatal(err)
	}
	if campaign.PricingModel != models.PricingCPC {
		t.Fatal("explicit CPC did not replace CPM")
	}
	for _, invalid := range []models.PricingModel{"cpv", "CPM", "unknown"} {
		req.PricingModel = invalid
		_, err := service.Create(
			ctx,
			req,
		)
		if !errors.Is(
			err,
			ErrInvalidInput,
		) {
			t.Fatalf(
				"create with pricing %q returned %v",
				invalid,
				err,
			)
		}
		_, err = service.Update(
			ctx,
			campaign.ID,
			req,
		)
		if !errors.Is(
			err,
			ErrInvalidInput,
		) {
			t.Fatalf(
				"update with pricing %q returned %v",
				invalid,
				err,
			)
		}
	}
}

func TestServiceCRUD(t *testing.T) {
	service := NewService(NewMemoryRepository())
	ctx := context.Background()

	created, err := service.Create(ctx, CampaignRequest{
		Name:   "Campaign One",
		Slug:   "campaign-one",
		Status: models.StatusPaused,
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
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("delete before archive error = %v, want ErrInvalidInput", err)
	}
	_, err = service.Update(
		ctx,
		created.ID,
		CampaignRequest{
			Name:   updated.Name,
			Slug:   updated.Slug,
			Status: models.StatusArchived,
		},
	)
	if err != nil {
		t.Fatal(err)
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

func TestTrackingParamsUpdatePreservesCampaignAndRejectsDuplicates(t *testing.T) {
	service := NewService(NewMemoryRepository())
	ctx := context.Background()
	created, err := service.Create(
		ctx,
		CampaignRequest{
			Name:   "Tracking campaign",
			Slug:   "tracking-campaign",
			Status: models.StatusPaused,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	params := []models.TrackingParam{
		{Key: "sub1", Value: " [ZONE_ID] "},
		{Key: "utm_content", Value: "[CLICK_ID]"},
	}
	updated, err := service.UpdateTrackingParams(
		ctx,
		created.ID,
		params,
	)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != created.Name || updated.Slug != created.Slug ||
		len(updated.TrackingParams) != 2 || updated.TrackingParams[0].Value != "[ZONE_ID]" {
		t.Fatalf("unexpected campaign after tracking update: %#v", updated)
	}
	_, err = service.UpdateTrackingParams(
		ctx,
		created.ID,
		[]models.TrackingParam{
			{Key: "sub1", Value: "a"},
			{Key: "sub1", Value: "b"},
		},
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("duplicate params error = %v, want ErrInvalidInput", err)
	}
	got, err := service.Get(
		ctx,
		created.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.TrackingParams) != 2 {
		t.Fatalf("invalid update changed saved params: %#v", got.TrackingParams)
	}
}

func TestServiceSavesAndUpdatesTrafficback(t *testing.T) {
	service := NewService(NewMemoryRepository())
	ctx := context.Background()
	created, err := service.Create(
		ctx,
		CampaignRequest{
			Name: "Trafficback campaign",
			Slug: "trafficback-campaign",
			Trafficback: &models.TrafficbackConfig{
				Enabled:  true,
				URL:      " https://example.org/fallback?depth={trafficback_depth} ",
				MaxDepth: 3,
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"create campaign: %v",
			err,
		)
	}
	if !created.TrafficbackConfig.Enabled ||
		created.TrafficbackConfig.URL != "https://example.org/fallback?depth={trafficback_depth}" ||
		created.TrafficbackConfig.MaxDepth != 3 {
		t.Fatalf(
			"unexpected created trafficback config: %#v",
			created.TrafficbackConfig,
		)
	}

	updated, err := service.Update(
		ctx,
		created.ID,
		CampaignRequest{
			Name: "Trafficback campaign",
			Slug: "trafficback-campaign",
		},
	)
	if err != nil {
		t.Fatalf(
			"update campaign without trafficback field: %v",
			err,
		)
	}
	if !updated.TrafficbackConfig.Enabled {
		t.Fatal("omitted trafficback field cleared existing config")
	}

	updated, err = service.Update(
		ctx,
		created.ID,
		CampaignRequest{
			Name:        "Trafficback campaign",
			Slug:        "trafficback-campaign",
			Status:      models.StatusPaused,
			Trafficback: &models.TrafficbackConfig{},
		},
	)
	if err != nil {
		t.Fatalf(
			"disable trafficback: %v",
			err,
		)
	}
	if updated.TrafficbackConfig.Enabled || updated.TrafficbackConfig.URL != "" {
		t.Fatalf(
			"trafficback remained enabled: %#v",
			updated.TrafficbackConfig,
		)
	}
}

func TestActiveCampaignRequiresTrafficback(t *testing.T) {
	service := NewService(NewMemoryRepository())
	ctx := context.Background()
	_, err := service.Create(
		ctx,
		CampaignRequest{
			Name:   "No fallback",
			Slug:   "no-fallback",
			Status: models.StatusActive,
		},
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("create active campaign without trafficback: %v", err)
	}
	created, err := service.Create(
		ctx,
		CampaignRequest{
			Name:   "Paused campaign",
			Slug:   "paused-campaign",
			Status: models.StatusPaused,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Update(
		ctx,
		created.ID,
		CampaignRequest{
			Name:   created.Name,
			Slug:   created.Slug,
			Status: models.StatusActive,
		},
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("activate campaign without trafficback: %v", err)
	}
}

func TestServiceRejectsInvalidTrafficback(t *testing.T) {
	service := NewService(NewMemoryRepository())
	for _, config := range []models.TrafficbackConfig{
		{Enabled: true, URL: "", MaxDepth: 3},
		{Enabled: true, URL: "javascript:alert(1)", MaxDepth: 3},
		{Enabled: true, URL: "https://example.org/fallback", MaxDepth: 0},
		{Enabled: true, URL: "https://example.org/fallback", MaxDepth: 11},
	} {
		_, err := service.Create(
			context.Background(),
			CampaignRequest{
				Name:        "Invalid trafficback",
				Slug:        "invalid-trafficback",
				Trafficback: &config,
			},
		)
		if !errors.Is(
			err,
			ErrInvalidInput,
		) {
			t.Fatalf(
				"config %#v returned %v, want ErrInvalidInput",
				config,
				err,
			)
		}
	}
}
