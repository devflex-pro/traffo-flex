package rules

import (
	"errors"
	"testing"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestMatchStreamReturnsFirstActiveMatchingStream(t *testing.T) {
	engine := NewEngine()
	streams := []models.Stream{
		{
			ID:     "str_paused",
			Status: models.StatusPaused,
			Conditions: []models.Condition{
				{Field: "geo_country", Operator: models.OperatorEQ, Value: "US"},
			},
		},
		{
			ID:     "str_match",
			Status: models.StatusActive,
			Conditions: []models.Condition{
				{Field: "geo_country", Operator: models.OperatorEQ, Value: "US"},
				{Field: "device_type", Operator: models.OperatorIn, Values: []string{"mobile", "tablet"}},
			},
		},
		{
			ID:     "str_late",
			Status: models.StatusActive,
		},
	}

	stream, err := engine.MatchStream(streams, Context{Values: map[string]string{
		"geo_country": "US",
		"device_type": "mobile",
	}})
	if err != nil {
		t.Fatalf(
			"MatchStream returned error: %v",
			err,
		)
	}
	if stream.ID != "str_match" {
		t.Fatalf(
			"stream id = %q, want str_match",
			stream.ID,
		)
	}
}

func TestMatchStreamReturnsNoMatch(t *testing.T) {
	engine := NewEngine()
	_, err := engine.MatchStream([]models.Stream{
		{
			ID:     "str_1",
			Status: models.StatusActive,
			Conditions: []models.Condition{
				{Field: "geo_country", Operator: models.OperatorEQ, Value: "US"},
			},
		},
	}, Context{Values: map[string]string{"geo_country": "DE"}})

	if !errors.Is(
		err,
		ErrNoMatchingStream,
	) {
		t.Fatalf(
			"error = %v, want ErrNoMatchingStream",
			err,
		)
	}
}

func TestConditionOperators(t *testing.T) {
	ctx := Context{Values: map[string]string{
		"sub1":  "zone-123",
		"cost":  "1.25",
		"empty": "",
	}}

	tests := []models.Condition{
		{Field: "sub1", Operator: models.OperatorContains, Value: "zone"},
		{Field: "sub1", Operator: models.OperatorStartsWith, Value: "zone"},
		{Field: "sub1", Operator: models.OperatorEndsWith, Value: "123"},
		{Field: "cost", Operator: models.OperatorGTE, Value: "1.25"},
		{Field: "sub1", Operator: models.OperatorRegex, Value: `^zone-\d+$`},
		{Field: "missing", Operator: models.OperatorNotExists},
	}

	for _, condition := range tests {
		if !matchCondition(
			condition,
			ctx,
		) {
			t.Fatalf(
				"condition %#v did not match",
				condition,
			)
		}
	}
}
