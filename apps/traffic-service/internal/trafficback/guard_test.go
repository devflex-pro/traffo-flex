package trafficback

import (
	"errors"
	"net/http/httptest"
	"testing"
)

func TestGuardParse(t *testing.T) {
	guard := NewGuard(3)
	req := httptest.NewRequest(
		"GET",
		"/tb/demo?trafficback_depth=2&trafficback_chain=dst_1,dst_2&visited_destinations=dst_1,dst_2",
		nil,
	)

	state := guard.Parse(req)

	if state.Depth != 2 {
		t.Fatalf(
			"Depth = %d, want 2",
			state.Depth,
		)
	}
	if len(state.Chain) != 2 {
		t.Fatalf(
			"len(Chain) = %d, want 2",
			len(state.Chain),
		)
	}
}

func TestGuardCheckDetectsMaxDepth(t *testing.T) {
	guard := NewGuard(2)

	err := guard.Check(
		State{Depth: 2},
		"dst_3",
	)
	if !errors.Is(
		err,
		ErrLoopDetected,
	) {
		t.Fatalf(
			"error = %v, want ErrLoopDetected",
			err,
		)
	}
}

func TestGuardCheckDetectsVisitedDestination(t *testing.T) {
	guard := NewGuard(3)

	err := guard.Check(
		State{Depth: 1, VisitedDestinations: []string{"dst_1"}},
		"dst_1",
	)
	if !errors.Is(
		err,
		ErrLoopDetected,
	) {
		t.Fatalf(
			"error = %v, want ErrLoopDetected",
			err,
		)
	}
}

func TestGuardNext(t *testing.T) {
	guard := NewGuard(3)

	next := guard.Next(
		State{Depth: 1, Chain: []string{"dst_1"}, VisitedDestinations: []string{"dst_1"}},
		"dst_2",
	)

	if next.Depth != 2 {
		t.Fatalf(
			"Depth = %d, want 2",
			next.Depth,
		)
	}
	if len(next.VisitedDestinations) != 2 {
		t.Fatalf(
			"len(VisitedDestinations) = %d, want 2",
			len(next.VisitedDestinations),
		)
	}
}
