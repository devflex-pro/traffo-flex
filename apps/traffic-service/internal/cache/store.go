package cache

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var ErrCampaignNotFound = errors.New("campaign not found")
var ErrDestinationNotFound = errors.New("destination not found")

type CampaignConfig struct {
	Campaign     models.Campaign      `json:"campaign"`
	Streams      []models.Stream      `json:"streams"`
	Destinations []models.Destination `json:"destinations"`
	LoadedAt     time.Time            `json:"loaded_at"`
}

type Loader interface {
	Load(ctx context.Context) (
		[]CampaignConfig,
		error,
	)
}

type StaticLoader struct {
	Campaigns []CampaignConfig
}

func (l StaticLoader) Load(ctx context.Context) (
	[]CampaignConfig,
	error,
) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	campaigns := make(
		[]CampaignConfig,
		len(l.Campaigns),
	)
	copy(
		campaigns,
		l.Campaigns,
	)
	return campaigns, nil
}

type Store struct {
	mu               sync.RWMutex
	reloadMu         sync.Mutex
	loader           Loader
	snapshot         *SnapshotFile
	bySlug           map[string]CampaignConfig
	byPublicID       map[string]CampaignConfig
	byToken          map[string]CampaignConfig
	reloadedAt       time.Time
	lastMongoSuccess time.Time
	lastMongoError   string
	source           string
}

func NewStore(loader Loader) *Store {
	if loader == nil {
		loader = StaticLoader{Campaigns: DemoCampaigns()}
	}
	return &Store{
		loader:     loader,
		bySlug:     make(map[string]CampaignConfig),
		byPublicID: make(map[string]CampaignConfig),
		byToken:    make(map[string]CampaignConfig),
	}
}

func NewPersistentStore(loader Loader, snapshot *SnapshotFile) *Store {
	store := NewStore(loader)
	store.snapshot = snapshot
	return store
}

func NewDemoStore() *Store {
	store := NewStore(StaticLoader{Campaigns: DemoCampaigns()})
	if err := store.Reload(context.Background()); err != nil {
		return NewStore(nil)
	}
	return store
}

func (s *Store) Reload(ctx context.Context) error {
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	campaigns, err := s.loader.Load(ctx)
	if err != nil {
		s.recordMongoError(err)
		return err
	}
	loadedAt := time.Now().UTC()
	if s.snapshot != nil {
		if err := s.snapshot.Save(campaigns, loadedAt); err != nil {
			s.recordMongoError(err)
			return err
		}
	}
	s.install(campaigns, loadedAt, "mongo")
	return nil
}

func (s *Store) Restore() error {
	if s.snapshot == nil {
		return ErrNoRoutingSnapshot
	}
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	campaigns, savedAt, err := s.snapshot.Load()
	if err != nil {
		return err
	}
	if len(campaigns) == 0 {
		return errors.New("routing snapshot has no active campaigns")
	}
	s.install(campaigns, savedAt, "snapshot")
	return nil
}

func (s *Store) install(campaigns []CampaignConfig, loadedAt time.Time, source string) {
	nextBySlug := make(
		map[string]CampaignConfig,
		len(campaigns),
	)
	nextByPublicID := make(
		map[string]CampaignConfig,
		len(campaigns),
	)
	nextByToken := make(
		map[string]CampaignConfig,
		len(campaigns),
	)
	for _, campaign := range campaigns {
		campaign.LoadedAt = loadedAt
		nextBySlug[campaign.Campaign.Slug] = campaign
		if campaign.Campaign.PublicID != "" {
			nextByPublicID[campaign.Campaign.PublicID] = campaign
		}
		if campaign.Campaign.PublicToken != "" {
			nextByToken[campaign.Campaign.PublicToken] = campaign
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.bySlug = nextBySlug
	s.byPublicID = nextByPublicID
	s.byToken = nextByToken
	s.reloadedAt = loadedAt
	s.source = source
	if source == "mongo" {
		s.lastMongoSuccess = loadedAt
		s.lastMongoError = ""
	}
}

func (s *Store) recordMongoError(err error) {
	s.mu.Lock()
	s.lastMongoError = err.Error()
	s.mu.Unlock()
}

type Status struct {
	Source           string    `json:"source"`
	Campaigns        int       `json:"campaigns"`
	LoadedAt         time.Time `json:"loaded_at,omitempty"`
	LastMongoSuccess time.Time `json:"last_mongo_success,omitempty"`
	LastMongoError   string    `json:"last_mongo_error,omitempty"`
}

func (s *Store) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Status{
		Source:           s.source,
		Campaigns:        len(s.bySlug),
		LoadedAt:         s.reloadedAt,
		LastMongoSuccess: s.lastMongoSuccess,
		LastMongoError:   s.lastMongoError,
	}
}

func (s *Store) Ready(r *http.Request) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.bySlug) == 0 {
		return errors.New("no active routing campaigns are loaded")
	}
	return nil
}

func (s *Store) GetBySlug(slug string) (
	CampaignConfig,
	error,
) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	campaign, ok := s.bySlug[slug]
	if !ok {
		return CampaignConfig{}, ErrCampaignNotFound
	}
	return campaign, nil
}

func (s *Store) GetByPublicID(publicID string) (
	CampaignConfig,
	error,
) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	campaign, ok := s.byPublicID[publicID]
	if !ok {
		return CampaignConfig{}, ErrCampaignNotFound
	}
	return campaign, nil
}

func (s *Store) GetByToken(token string) (
	CampaignConfig,
	error,
) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	campaign, ok := s.byToken[token]
	if !ok {
		return CampaignConfig{}, ErrCampaignNotFound
	}
	return campaign, nil
}

func (s *Store) GetDestination(id string) (
	models.Destination,
	error,
) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, campaign := range s.bySlug {
		for _, destination := range campaign.Destinations {
			if destination.ID == id {
				return destination, nil
			}
		}
	}
	return models.Destination{}, ErrDestinationNotFound
}

func (s *Store) DestinationIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]struct{})
	ids := make([]string, 0)
	for _, campaign := range s.bySlug {
		for _, destination := range campaign.Destinations {
			if _, exists := seen[destination.ID]; exists {
				continue
			}
			seen[destination.ID] = struct{}{}
			ids = append(
				ids,
				destination.ID,
			)
		}
	}
	return ids
}

func (s *Store) SnapshotCampaigns() []CampaignConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	campaigns := make([]CampaignConfig, 0, len(s.bySlug))
	for _, campaign := range s.bySlug {
		copyOfCampaign := campaign
		copyOfCampaign.Streams = append([]models.Stream(nil), campaign.Streams...)
		copyOfCampaign.Destinations = append([]models.Destination(nil), campaign.Destinations...)
		campaigns = append(campaigns, copyOfCampaign)
	}
	return campaigns
}

func (s *Store) UpdateDestinationHealth(
	id string,
	status models.HealthStatus,
	updatedAt time.Time,
) (
	models.Destination,
	error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	destination, updated := updateDestinationHealth(
		s.bySlug,
		id,
		status,
		updatedAt,
	)
	if !updated {
		return models.Destination{}, ErrDestinationNotFound
	}
	updateDestinationHealth(
		s.byPublicID,
		id,
		status,
		updatedAt,
	)
	updateDestinationHealth(
		s.byToken,
		id,
		status,
		updatedAt,
	)
	return destination, nil
}

func (s *Store) ReloadedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reloadedAt
}

func updateDestinationHealth(
	campaigns map[string]CampaignConfig,
	id string,
	status models.HealthStatus,
	updatedAt time.Time,
) (
	models.Destination,
	bool,
) {
	for key, campaign := range campaigns {
		for index, destination := range campaign.Destinations {
			if destination.ID != id {
				continue
			}
			destination.HealthStatus = status
			destination.UpdatedAt = updatedAt
			campaign.Destinations[index] = destination
			campaigns[key] = campaign
			return destination, true
		}
	}
	return models.Destination{}, false
}

func DemoCampaigns() []CampaignConfig {
	now := time.Now().UTC()
	return []CampaignConfig{
		{
			Campaign: models.Campaign{
				ID:          "cmp_demo",
				PublicID:    "demo-public",
				PublicToken: "demo-token",
				Name:        "Demo Campaign",
				Slug:        "demo",
				Status:      models.StatusActive,
				TrafficbackConfig: models.TrafficbackConfig{
					Enabled:  true,
					URL:      "https://example.com/trafficback?click_id={click_id}&reason={trafficback_reason}&trafficback_depth={trafficback_depth}",
					MaxDepth: 3,
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
			Streams: []models.Stream{
				{
					ID:         "str_demo",
					CampaignID: "cmp_demo",
					Name:       "Demo Stream",
					Priority:   0,
					Status:     models.StatusActive,
					Distribution: models.Distribution{
						Mode: models.DistributionWeighted,
						Destinations: []models.WeightedTarget{
							{DestinationID: "dst_demo", Weight: 100},
						},
					},
					CreatedAt: now,
					UpdatedAt: now,
				},
			},
			Destinations: []models.Destination{
				{
					ID:           "dst_demo",
					Name:         "Demo Destination",
					Type:         models.DestinationURL,
					URL:          "https://example.com/?subid={click_id}",
					ManualStatus: models.StatusActive,
					HealthStatus: models.HealthUnknown,
					Redirect:     models.RedirectConfig{Mode: models.RedirectHTTP302},
					CreatedAt:    now,
					UpdatedAt:    now,
				},
			},
		},
	}
}
