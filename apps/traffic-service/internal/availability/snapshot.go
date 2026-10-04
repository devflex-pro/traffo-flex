package availability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var ErrCapSnapshotMissing = errors.New("destination cap snapshot is missing")

type SnapshotStats struct {
	LastAttemptAt    time.Time `json:"last_attempt_at,omitempty"`
	LastSuccessAt    time.Time `json:"last_success_at,omitempty"`
	AgeSeconds       int64     `json:"age_seconds"`
	LastError        string    `json:"last_error,omitempty"`
	RefreshFailures  uint64    `json:"refresh_failures"`
	MissingCapChecks uint64    `json:"missing_cap_checks"`
	CapRules         int       `json:"cap_rules"`
	ROIPolicies      int       `json:"roi_policies"`
}

type roiPolicy struct {
	windowHours int
	minClicks   int
}

type Snapshot struct {
	store  *cache.Store
	source interface {
		CapChecker
		ROIRanker
	}
	log       *slog.Logger
	interval  time.Duration
	refreshMu sync.Mutex
	mu        sync.RWMutex
	missing   atomic.Uint64
	caps      map[string]bool
	roi       map[roiPolicy][]string
	stats     SnapshotStats
}

func NewSnapshot(
	store *cache.Store,
	source interface {
		CapChecker
		ROIRanker
	},
	log *slog.Logger,
	interval time.Duration,
) *Snapshot {
	return &Snapshot{
		store:    store,
		source:   source,
		log:      log,
		interval: interval,
		caps:     make(map[string]bool),
		roi:      make(map[roiPolicy][]string),
	}
}

func (s *Snapshot) Start(ctx context.Context) {
	go func() {
		if err := s.Refresh(ctx); err != nil && s.log != nil {
			s.log.Warn("availability snapshot refresh failed", "error", err)
		}
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.Refresh(ctx); err != nil && s.log != nil {
					s.log.Warn("availability snapshot refresh failed", "error", err)
				}
			}
		}
	}()
}

func (s *Snapshot) Refresh(ctx context.Context) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	campaigns := s.store.SnapshotCampaigns()
	destinations := make(map[string]models.Destination)
	policies := make(map[roiPolicy]struct{})
	for _, campaign := range campaigns {
		if campaign.Campaign.Status != models.StatusActive {
			continue
		}
		byID := make(map[string]models.Destination, len(campaign.Destinations))
		for _, destination := range campaign.Destinations {
			byID[destination.ID] = destination
		}
		for _, stream := range campaign.Streams {
			if stream.Status != models.StatusActive {
				continue
			}
			for _, target := range stream.Distribution.Destinations {
				if destination, ok := byID[target.DestinationID]; ok {
					destinations[destination.ID] = destination
				}
			}
			policy := stream.Distribution.UniquePolicy
			if policy.SelectionStrategy == models.DistributionBestROI {
				policies[roiPolicy{policy.ROIWindowHours, policy.MinClicks}] = struct{}{}
			}
		}
	}
	ids := make([]string, 0, len(destinations))
	for id := range destinations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	s.mu.RLock()
	previousCaps := make(map[string]bool, len(s.caps))
	for key, value := range s.caps {
		previousCaps[key] = value
	}
	previousROI := make(map[roiPolicy][]string, len(s.roi))
	for key, value := range s.roi {
		previousROI[key] = value
	}
	s.mu.RUnlock()
	caps := make(map[string]bool)
	roi := make(map[roiPolicy][]string)
	now := time.Now().UTC()
	var failures []error
	for _, id := range ids {
		destination := destinations[id]
		if !destination.Caps.Enabled {
			continue
		}
		for _, rule := range destination.Caps.Rules {
			key := cacheKey(id, rule)
			singleRule := destination
			singleRule.Caps.Rules = []models.DestinationCapRule{rule}
			exceeded, err := s.source.Exceeded(ctx, singleRule, now)
			if err != nil {
				failures = append(failures, err)
				if previous, ok := previousCaps[key]; ok {
					caps[key] = previous
				}
				continue
			}
			caps[key] = exceeded
		}
	}
	for policy := range policies {
		ranked, err := s.source.RankByROI(
			ctx,
			ids,
			policy.windowHours,
			policy.minClicks,
			now,
		)
		if err != nil {
			failures = append(failures, err)
			if previous, ok := previousROI[policy]; ok {
				roi[policy] = previous
			}
			continue
		}
		roi[policy] = ranked
	}
	s.mu.Lock()
	s.caps = caps
	s.roi = roi
	s.stats.LastAttemptAt = now
	s.stats.CapRules = len(caps)
	s.stats.ROIPolicies = len(roi)
	if len(failures) == 0 {
		s.stats.LastSuccessAt = now
		s.stats.LastError = ""
	} else {
		s.stats.RefreshFailures++
		s.stats.LastError = fmt.Sprintf(
			"%d refresh queries failed; first: %v",
			len(failures),
			failures[0],
		)
	}
	s.mu.Unlock()
	if len(failures) > 0 {
		return fmt.Errorf(
			"availability refresh: %d queries failed: %w",
			len(failures),
			failures[0],
		)
	}
	return nil
}

func (s *Snapshot) Exceeded(
	ctx context.Context,
	destination models.Destination,
	now time.Time,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rule := range destination.Caps.Rules {
		exceeded, ok := s.caps[cacheKey(destination.ID, rule)]
		if !ok {
			s.missing.Add(1)
			return false, ErrCapSnapshotMissing
		}
		if exceeded {
			return true, nil
		}
	}
	return false, nil
}

func (s *Snapshot) RankByROI(
	ctx context.Context,
	destinationIDs []string,
	windowHours int,
	minClicks int,
	now time.Time,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	ranked, ok := s.roi[roiPolicy{windowHours, minClicks}]
	s.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	available := make(map[string]struct{}, len(destinationIDs))
	for _, id := range destinationIDs {
		available[id] = struct{}{}
	}
	result := make([]string, 0, len(destinationIDs))
	for _, id := range ranked {
		if _, ok := available[id]; ok {
			result = append(result, id)
		}
	}
	return result, nil
}

func (s *Snapshot) Stats() SnapshotStats {
	s.mu.RLock()
	stats := s.stats
	s.mu.RUnlock()
	stats.MissingCapChecks = s.missing.Load()
	if !stats.LastSuccessAt.IsZero() {
		stats.AgeSeconds = int64(time.Since(stats.LastSuccessAt).Seconds())
	}
	return stats
}
