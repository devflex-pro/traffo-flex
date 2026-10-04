package cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type failingLoader struct{}

func (failingLoader) Load(ctx context.Context) ([]CampaignConfig, error) {
	return nil, errors.New("mongo unavailable")
}

type switchLoader struct {
	available bool
	campaigns []CampaignConfig
}

func (l *switchLoader) Load(ctx context.Context) ([]CampaignConfig, error) {
	if !l.available {
		return nil, errors.New("mongo unavailable")
	}
	return l.campaigns, nil
}

func TestRestoreRoutesDuringMongoOutage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaigns.json")
	snapshot := NewSnapshotFile(path)
	initial := NewPersistentStore(
		StaticLoader{Campaigns: DemoCampaigns()},
		snapshot,
	)
	if err := initial.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	restarted := NewPersistentStore(
		failingLoader{},
		snapshot,
	)
	if err := restarted.Restore(); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Reload(context.Background()); err == nil {
		t.Fatal("expected MongoDB refresh to fail")
	}
	if _, err := restarted.GetBySlug("demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.GetByPublicID("demo-public"); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.GetByToken("demo-token"); err != nil {
		t.Fatal(err)
	}
	if status := restarted.Status(); status.Source != "snapshot" || status.Campaigns != 1 || status.LastMongoError == "" {
		t.Fatalf("unexpected cache status: %+v", status)
	}
}

func TestCorruptSnapshotIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaigns.json")
	snapshot := NewSnapshotFile(path)
	store := NewPersistentStore(
		StaticLoader{Campaigns: DemoCampaigns()},
		snapshot,
	)
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"checksum":"bad","payload":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	restarted := NewPersistentStore(failingLoader{}, snapshot)
	if err := restarted.Restore(); err == nil {
		t.Fatal("expected corrupt snapshot to be rejected")
	}
	if _, err := restarted.GetBySlug("demo"); !errors.Is(err, ErrCampaignNotFound) {
		t.Fatalf("unexpected route after corrupt snapshot: %v", err)
	}
}

func TestSnapshotRejectsCampaignWithoutTrafficback(t *testing.T) {
	campaigns := DemoCampaigns()
	campaigns[0].Campaign.TrafficbackConfig.Enabled = false
	store := NewPersistentStore(
		StaticLoader{Campaigns: campaigns},
		NewSnapshotFile(filepath.Join(t.TempDir(), "campaigns.json")),
	)
	if err := store.Reload(context.Background()); err == nil {
		t.Fatal("expected campaign without trafficback to be rejected")
	}
	if _, err := store.GetBySlug("demo"); !errors.Is(err, ErrCampaignNotFound) {
		t.Fatalf("invalid campaign became active: %v", err)
	}
}

func TestSnapshotRejectsHealthcheckURLWithMacros(t *testing.T) {
	campaigns := DemoCampaigns()
	campaigns[0].Destinations[0].HealthcheckURL = "https://example.com/health?click_id={click_id}"
	store := NewPersistentStore(
		StaticLoader{Campaigns: campaigns},
		NewSnapshotFile(filepath.Join(t.TempDir(), "campaigns.json")),
	)
	if err := store.Reload(context.Background()); err == nil {
		t.Fatal("expected healthcheck URL with macros to be rejected")
	}
}

func TestInvalidRefreshRetainsLastGoodRoute(t *testing.T) {
	loader := &switchLoader{available: true, campaigns: DemoCampaigns()}
	store := NewPersistentStore(
		loader,
		NewSnapshotFile(filepath.Join(t.TempDir(), "campaigns.json")),
	)
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	invalid := DemoCampaigns()
	invalid[0].Campaign.TrafficbackConfig.URL = ""
	loader.campaigns = invalid
	if err := store.Reload(context.Background()); err == nil {
		t.Fatal("expected invalid refresh to fail")
	}
	campaign, err := store.GetBySlug("demo")
	if err != nil || campaign.Campaign.TrafficbackConfig.URL == "" {
		t.Fatalf("last good route was lost: %v", err)
	}
}

func TestMongoRecoveryReplacesSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaigns.json")
	snapshot := NewSnapshotFile(path)
	seed := NewPersistentStore(
		StaticLoader{Campaigns: DemoCampaigns()},
		snapshot,
	)
	if err := seed.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	updated := DemoCampaigns()
	updated[0].Destinations[0].URL = "https://example.org/new"
	loader := &switchLoader{campaigns: updated}
	store := NewPersistentStore(loader, snapshot)
	if err := store.Restore(); err != nil {
		t.Fatal(err)
	}
	if err := store.Reload(context.Background()); err == nil {
		t.Fatal("expected MongoDB to be unavailable")
	}
	loader.available = true
	if err := store.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if status := store.Status(); status.Source != "mongo" || status.LastMongoError != "" {
		t.Fatalf("expected recovered MongoDB cache, got %+v", status)
	}
	campaign, err := store.GetBySlug("demo")
	if err != nil || campaign.Destinations[0].URL != "https://example.org/new" {
		t.Fatalf("expected updated route, got %+v, %v", campaign, err)
	}
	restarted := NewPersistentStore(failingLoader{}, snapshot)
	if err := restarted.Restore(); err != nil {
		t.Fatal(err)
	}
	campaign, err = restarted.GetBySlug("demo")
	if err != nil || campaign.Destinations[0].URL != "https://example.org/new" {
		t.Fatalf("expected updated snapshot, got %+v, %v", campaign, err)
	}
}
