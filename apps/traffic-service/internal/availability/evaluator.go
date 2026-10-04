package availability

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type CapChecker interface {
	Exceeded(
		ctx context.Context,
		destination models.Destination,
		now time.Time,
	) (
		bool,
		error,
	)
}

type HistoryChecker interface {
	UsedDestinations(
		ctx context.Context,
		userKey string,
		userValue string,
		windowHours int,
		now time.Time,
	) (
		map[string]struct{},
		error,
	)
}

type ROIRanker interface {
	RankByROI(
		ctx context.Context,
		destinationIDs []string,
		windowHours int,
		minClicks int,
		now time.Time,
	) (
		[]string,
		error,
	)
}

type Evaluator struct {
	log     *slog.Logger
	caps    CapChecker
	history HistoryChecker
	roi     ROIRanker
	nowFunc func() time.Time
}

func NewEvaluator(
	log *slog.Logger,
	caps CapChecker,
) *Evaluator {
	return &Evaluator{
		log:     log,
		caps:    caps,
		history: asHistoryChecker(caps),
		roi:     asROIRanker(caps),
		nowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func NewEvaluatorWithSources(
	log *slog.Logger,
	caps CapChecker,
	history HistoryChecker,
	roi ROIRanker,
) *Evaluator {
	return &Evaluator{
		log:     log,
		caps:    caps,
		history: history,
		roi:     roi,
		nowFunc: func() time.Time { return time.Now().UTC() },
	}
}

func NewEvaluatorWithClock(
	log *slog.Logger,
	caps CapChecker,
	nowFunc func() time.Time,
) *Evaluator {
	evaluator := NewEvaluator(
		log,
		caps,
	)
	if nowFunc != nil {
		evaluator.nowFunc = nowFunc
	}
	return evaluator
}

func (e *Evaluator) Filter(
	ctx context.Context,
	distribution models.Distribution,
	destinations []models.Destination,
	values map[string]string,
	selectionKey string,
) []models.Destination {
	if e == nil {
		return destinations
	}
	now := e.nowFunc()
	available := e.filterAvailable(
		ctx,
		destinations,
		now,
	)
	return e.applyUniquePolicy(
		ctx,
		distribution,
		available,
		values,
		selectionKey,
		now,
	)
}

func (e *Evaluator) filterAvailable(
	ctx context.Context,
	destinations []models.Destination,
	now time.Time,
) []models.Destination {
	available := make(
		[]models.Destination,
		0,
		len(destinations),
	)
	for _, destination := range destinations {
		if !destination.AvailableAt(now) {
			continue
		}
		exceeded, err := e.capExceeded(
			ctx,
			destination,
			now,
		)
		if err != nil {
			if errors.Is(err, ErrCapSnapshotMissing) {
				continue
			}
			e.warn(
				"destination cap check failed",
				"destination_id",
				destination.ID,
				"error",
				err,
			)
			available = append(
				available,
				destination,
			)
			continue
		}
		if exceeded {
			e.warn(
				"destination cap reached",
				"destination_id",
				destination.ID,
			)
			continue
		}
		available = append(
			available,
			destination,
		)
	}
	return available
}

func (e *Evaluator) applyUniquePolicy(
	ctx context.Context,
	distribution models.Distribution,
	destinations []models.Destination,
	values map[string]string,
	selectionKey string,
	now time.Time,
) []models.Destination {
	policy := normalizePolicy(distribution.UniquePolicy)
	if !policy.Enabled || e.history == nil {
		return e.rankDestinations(
			ctx,
			policy,
			destinations,
			selectionKey,
			now,
		)
	}
	userValue := values[policy.UserKey]
	if userValue == "" {
		return e.rankDestinations(
			ctx,
			policy,
			destinations,
			selectionKey,
			now,
		)
	}
	used, err := e.history.UsedDestinations(
		ctx,
		policy.UserKey,
		userValue,
		policy.HistoryWindowHours,
		now,
	)
	if err != nil {
		e.warn(
			"destination history check failed",
			"user_key",
			policy.UserKey,
			"error",
			err,
		)
		return e.rankDestinations(
			ctx,
			policy,
			destinations,
			selectionKey,
			now,
		)
	}
	filtered := make(
		[]models.Destination,
		0,
		len(destinations),
	)
	for _, destination := range destinations {
		if _, ok := used[destination.ID]; ok {
			continue
		}
		filtered = append(
			filtered,
			destination,
		)
	}
	if len(filtered) == 0 && policy.ExhaustedMode == models.UniqueExhaustedAllowRepeat {
		filtered = destinations
	}
	return e.rankDestinations(
		ctx,
		policy,
		filtered,
		selectionKey,
		now,
	)
}

func (e *Evaluator) rankDestinations(
	ctx context.Context,
	policy models.UniqueDestinationPolicy,
	destinations []models.Destination,
	selectionKey string,
	now time.Time,
) []models.Destination {
	if policy.SelectionStrategy != models.DistributionBestROI || e.roi == nil || len(destinations) < 2 {
		return destinations
	}
	ids := make(
		[]string,
		0,
		len(destinations),
	)
	byID := make(
		map[string]models.Destination,
		len(destinations),
	)
	for _, destination := range destinations {
		ids = append(
			ids,
			destination.ID,
		)
		byID[destination.ID] = destination
	}
	rankedIDs, err := e.roi.RankByROI(
		ctx,
		ids,
		policy.ROIWindowHours,
		policy.MinClicks,
		now,
	)
	if err != nil {
		e.warn(
			"destination roi ranking failed",
			"error",
			err,
		)
		return destinations
	}
	if len(rankedIDs) == 0 {
		return fallbackRank(
			policy.FallbackStrategy,
			destinations,
			selectionKey,
		)
	}
	ranked := make(
		[]models.Destination,
		0,
		len(destinations),
	)
	seen := make(map[string]struct{})
	for _, id := range rankedIDs {
		destination, ok := byID[id]
		if !ok {
			continue
		}
		ranked = append(
			ranked,
			destination,
		)
		seen[id] = struct{}{}
	}
	for _, destination := range destinations {
		if _, ok := seen[destination.ID]; ok {
			continue
		}
		ranked = append(
			ranked,
			destination,
		)
	}
	return ranked
}

func fallbackRank(
	strategy models.DistributionMode,
	destinations []models.Destination,
	selectionKey string,
) []models.Destination {
	if strategy != models.DistributionRoundRobin && strategy != models.DistributionWeighted {
		return destinations
	}
	if len(destinations) < 2 {
		return destinations
	}
	offset := int(hashKey(selectionKey) % uint64(len(destinations)))
	ranked := make(
		[]models.Destination,
		0,
		len(destinations),
	)
	ranked = append(
		ranked,
		destinations[offset:]...,
	)
	ranked = append(
		ranked,
		destinations[:offset]...,
	)
	return ranked
}

func hashKey(key string) uint64 {
	var hash uint64 = 14695981039346656037
	for i := 0; i < len(key); i++ {
		hash ^= uint64(key[i])
		hash *= 1099511628211
	}
	return hash
}

func (e *Evaluator) capExceeded(
	ctx context.Context,
	destination models.Destination,
	now time.Time,
) (
	bool,
	error,
) {
	if e.caps == nil || !destination.Caps.Enabled {
		return false, nil
	}
	return e.caps.Exceeded(
		ctx,
		destination,
		now,
	)
}

func (e *Evaluator) warn(
	message string,
	args ...any,
) {
	if e.log == nil {
		return
	}
	e.log.Warn(
		message,
		args...,
	)
}

func normalizePolicy(policy models.UniqueDestinationPolicy) models.UniqueDestinationPolicy {
	if policy.ExhaustedMode == "" {
		policy.ExhaustedMode = models.UniqueExhaustedAllowRepeat
	}
	if policy.SelectionStrategy == "" {
		policy.SelectionStrategy = models.DistributionWaterfall
	}
	if policy.FallbackStrategy == "" {
		policy.FallbackStrategy = models.DistributionRoundRobin
	}
	return policy
}

func asHistoryChecker(value any) HistoryChecker {
	checker, ok := value.(HistoryChecker)
	if !ok {
		return nil
	}
	return checker
}

func asROIRanker(value any) ROIRanker {
	ranker, ok := value.(ROIRanker)
	if !ok {
		return nil
	}
	return ranker
}
