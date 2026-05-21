package trafficback

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
)

var ErrLoopDetected = errors.New("trafficback loop detected")

type Guard struct {
	MaxDepth int
}

type State struct {
	Depth               int
	Chain               []string
	VisitedDestinations []string
}

func NewGuard(maxDepth int) Guard {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	return Guard{MaxDepth: maxDepth}
}

func (g Guard) Parse(r *http.Request) State {
	q := r.URL.Query()
	depth, err := strconv.Atoi(q.Get("trafficback_depth"))
	if err != nil || depth < 0 {
		depth = 0
	}
	return State{
		Depth:               depth,
		Chain:               splitList(q.Get("trafficback_chain")),
		VisitedDestinations: splitList(q.Get("visited_destinations")),
	}
}

func (g Guard) Check(
	state State,
	nextDestinationID string,
) error {
	if state.Depth >= g.MaxDepth {
		return ErrLoopDetected
	}
	nextDestinationID = strings.TrimSpace(nextDestinationID)
	if nextDestinationID == "" {
		return nil
	}
	for _, visited := range state.VisitedDestinations {
		if visited == nextDestinationID {
			return ErrLoopDetected
		}
	}
	return nil
}

func (g Guard) Next(
	state State,
	destinationID string,
) State {
	destinationID = strings.TrimSpace(destinationID)
	next := State{
		Depth: state.Depth + 1,
		Chain: append(
			[]string{},
			state.Chain...,
		),
		VisitedDestinations: append(
			[]string{},
			state.VisitedDestinations...,
		),
	}
	if destinationID != "" {
		next.Chain = append(
			next.Chain,
			destinationID,
		)
		next.VisitedDestinations = append(
			next.VisitedDestinations,
			destinationID,
		)
	}
	return next
}

func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(
		value,
		",",
	)
	items := make(
		[]string,
		0,
		len(parts),
	)
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(
				items,
				item,
			)
		}
	}
	return items
}
