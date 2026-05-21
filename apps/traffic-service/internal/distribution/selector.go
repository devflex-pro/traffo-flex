package distribution

import (
	"errors"
	"hash/fnv"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var ErrNoDestination = errors.New("no destination selected")

type Selector struct{}

func NewSelector() *Selector {
	return &Selector{}
}

func (s *Selector) Select(
	distribution models.Distribution,
	destinations []models.Destination,
	key string,
) (
	models.Destination,
	error,
) {
	available := availableByID(destinations)
	if len(available) == 0 {
		return models.Destination{}, ErrNoDestination
	}

	switch distribution.Mode {
	case models.DistributionBestROI:
		return firstAvailableInDestinationOrder(destinations)
	case models.DistributionDirect, models.DistributionFallback, models.DistributionWaterfall:
		return firstConfiguredAvailable(
			distribution.Destinations,
			available,
		)
	case models.DistributionWeighted, models.DistributionRoundRobin:
		return weighted(
			distribution.Destinations,
			available,
			key,
		)
	default:
		return models.Destination{}, ErrNoDestination
	}
}

func firstAvailableInDestinationOrder(destinations []models.Destination) (
	models.Destination,
	error,
) {
	for _, destination := range destinations {
		if destination.Available() {
			return destination, nil
		}
	}
	return models.Destination{}, ErrNoDestination
}

func availableByID(destinations []models.Destination) map[string]models.Destination {
	available := make(map[string]models.Destination)
	for _, destination := range destinations {
		if destination.Available() {
			available[destination.ID] = destination
		}
	}
	return available
}

func firstConfiguredAvailable(
	targets []models.WeightedTarget,
	available map[string]models.Destination,
) (
	models.Destination,
	error,
) {
	for _, target := range targets {
		if destination, ok := available[target.DestinationID]; ok {
			return destination, nil
		}
	}
	return models.Destination{}, ErrNoDestination
}

func weighted(
	targets []models.WeightedTarget,
	available map[string]models.Destination,
	key string,
) (
	models.Destination,
	error,
) {
	total := 0
	for _, target := range targets {
		if _, ok := available[target.DestinationID]; ok && target.Weight > 0 {
			total += target.Weight
		}
	}
	if total == 0 {
		return models.Destination{}, ErrNoDestination
	}

	slot := int(hashKey(key) % uint64(total))
	cursor := 0
	for _, target := range targets {
		destination, ok := available[target.DestinationID]
		if !ok || target.Weight <= 0 {
			continue
		}
		cursor += target.Weight
		if slot < cursor {
			return destination, nil
		}
	}
	return models.Destination{}, ErrNoDestination
}

func hashKey(key string) uint64 {
	hash := fnv.New64a()
	if _, err := hash.Write([]byte(key)); err != nil {
		return 0
	}
	return hash.Sum64()
}
