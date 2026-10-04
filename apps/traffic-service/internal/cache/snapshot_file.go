package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var ErrNoRoutingSnapshot = errors.New("routing snapshot does not exist")

const routingSnapshotVersion = 1
const maxRoutingSnapshotBytes = 64 << 20

type SnapshotFile struct {
	path string
}

type routingSnapshot struct {
	Version   int              `json:"version"`
	SavedAt   time.Time        `json:"saved_at"`
	Campaigns []CampaignConfig `json:"campaigns"`
}

type snapshotEnvelope struct {
	Checksum string          `json:"checksum"`
	Payload  json.RawMessage `json:"payload"`
}

func NewSnapshotFile(path string) *SnapshotFile {
	return &SnapshotFile{path: path}
}

func (s *SnapshotFile) Save(campaigns []CampaignConfig, savedAt time.Time) (resultErr error) {
	if err := validateRoutingSnapshot(campaigns); err != nil {
		return err
	}
	payload, err := json.Marshal(routingSnapshot{
		Version:   routingSnapshotVersion,
		SavedAt:   savedAt,
		Campaigns: campaigns,
	})
	if err != nil {
		return err
	}
	checksum := sha256.Sum256(payload)
	encoded, err := json.Marshal(snapshotEnvelope{
		Checksum: hex.EncodeToString(checksum[:]),
		Payload:  payload,
	})
	if err != nil {
		return err
	}
	if len(encoded) > maxRoutingSnapshotBytes {
		return errors.New("routing snapshot exceeds size limit")
	}
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".routing-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer func() {
		if err := os.Remove(tempPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	if err := file.Chmod(0600); err != nil {
		return errors.Join(err, file.Close())
	}
	if _, err := file.Write(encoded); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		return err
	}
	parent, err := os.Open(directory)
	if err != nil {
		return err
	}
	return errors.Join(parent.Sync(), parent.Close())
}

func (s *SnapshotFile) Load() (campaigns []CampaignConfig, savedAt time.Time, resultErr error) {
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, time.Time{}, ErrNoRoutingSnapshot
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	encoded, err := io.ReadAll(io.LimitReader(file, maxRoutingSnapshotBytes+1))
	if err != nil {
		return nil, time.Time{}, err
	}
	if len(encoded) > maxRoutingSnapshotBytes {
		return nil, time.Time{}, errors.New("routing snapshot exceeds size limit")
	}
	var envelope snapshotEnvelope
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		return nil, time.Time{}, err
	}
	checksum := sha256.Sum256(envelope.Payload)
	if envelope.Checksum != hex.EncodeToString(checksum[:]) {
		return nil, time.Time{}, errors.New("routing snapshot checksum mismatch")
	}
	var snapshot routingSnapshot
	if err := json.Unmarshal(envelope.Payload, &snapshot); err != nil {
		return nil, time.Time{}, err
	}
	if snapshot.Version != routingSnapshotVersion || snapshot.SavedAt.IsZero() || snapshot.SavedAt.After(time.Now().Add(time.Minute)) {
		return nil, time.Time{}, errors.New("routing snapshot version or timestamp is invalid")
	}
	if err := validateRoutingSnapshot(snapshot.Campaigns); err != nil {
		return nil, time.Time{}, err
	}
	return snapshot.Campaigns, snapshot.SavedAt, nil
}

func validateRoutingSnapshot(campaigns []CampaignConfig) error {
	seenSlug := make(map[string]struct{}, len(campaigns))
	seenPublicID := make(map[string]struct{}, len(campaigns))
	seenToken := make(map[string]struct{}, len(campaigns))
	for _, campaign := range campaigns {
		if campaign.Campaign.ID == "" || campaign.Campaign.Status != models.StatusActive {
			return errors.New("routing snapshot contains inactive or unidentified campaign")
		}
		if err := models.ValidateSlug(campaign.Campaign.Slug); err != nil {
			return fmt.Errorf("campaign %s: %w", campaign.Campaign.ID, err)
		}
		if _, exists := seenSlug[campaign.Campaign.Slug]; exists {
			return errors.New("routing snapshot contains duplicate campaign slug")
		}
		seenSlug[campaign.Campaign.Slug] = struct{}{}
		if campaign.Campaign.PublicID != "" {
			if err := models.ValidatePublicID(campaign.Campaign.PublicID); err != nil {
				return err
			}
			if _, exists := seenPublicID[campaign.Campaign.PublicID]; exists {
				return errors.New("routing snapshot contains duplicate campaign public ID")
			}
			seenPublicID[campaign.Campaign.PublicID] = struct{}{}
		}
		if campaign.Campaign.PublicToken != "" {
			if _, exists := seenToken[campaign.Campaign.PublicToken]; exists {
				return errors.New("routing snapshot contains duplicate campaign public token")
			}
			seenToken[campaign.Campaign.PublicToken] = struct{}{}
		}
		if !campaign.Campaign.TrafficbackConfig.Enabled || campaign.Campaign.TrafficbackConfig.URL == "" {
			return fmt.Errorf("campaign %s has no configured trafficback", campaign.Campaign.ID)
		}
		if err := validateTrafficbackURL(campaign.Campaign.TrafficbackConfig); err != nil {
			return err
		}
		for _, stream := range campaign.Streams {
			if stream.CampaignID != campaign.Campaign.ID || stream.Status != models.StatusActive {
				return errors.New("routing snapshot contains mismatched or inactive stream")
			}
			if err := models.ValidateDistribution(stream.Distribution); err != nil {
				return err
			}
			if err := validateTrafficbackURL(stream.TrafficbackConfig); err != nil {
				return err
			}
		}
		for _, destination := range campaign.Destinations {
			if destination.ID == "" {
				return errors.New("routing snapshot contains unidentified destination")
			}
			if err := models.ValidateDestinationURL(destination.URL); err != nil {
				return fmt.Errorf("destination %s: %w", destination.ID, err)
			}
			if destination.HealthcheckURL != "" {
				if err := models.ValidateDestinationURL(destination.HealthcheckURL); err != nil {
					return fmt.Errorf("destination %s healthcheck: %w", destination.ID, err)
				}
				if strings.ContainsAny(destination.HealthcheckURL, "{}") {
					return fmt.Errorf("destination %s healthcheck URL contains macros", destination.ID)
				}
			}
		}
	}
	return nil
}

func validateTrafficbackURL(config models.TrafficbackConfig) error {
	for _, target := range []string{config.URL, config.FallbackURL} {
		if target == "" {
			continue
		}
		if err := models.ValidateDestinationURL(target); err != nil {
			return err
		}
	}
	return nil
}
