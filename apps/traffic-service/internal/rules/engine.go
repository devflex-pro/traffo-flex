package rules

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var ErrNoMatchingStream = errors.New("no matching stream")

type Context struct {
	Values map[string]string
}

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) MatchStream(
	streams []models.Stream,
	ctx Context,
) (
	models.Stream,
	error,
) {
	for _, stream := range streams {
		if stream.Status != models.StatusActive {
			continue
		}
		if matchConditions(
			stream.Conditions,
			ctx,
		) {
			return stream, nil
		}
	}
	return models.Stream{}, ErrNoMatchingStream
}

func matchConditions(
	conditions []models.Condition,
	ctx Context,
) bool {
	for _, condition := range conditions {
		if !matchCondition(
			condition,
			ctx,
		) {
			return false
		}
	}
	return true
}

func matchCondition(
	condition models.Condition,
	ctx Context,
) bool {
	value, exists := ctx.Values[condition.Field]
	switch condition.Operator {
	case models.OperatorExists:
		return exists && value != ""
	case models.OperatorNotExists:
		return !exists || value == ""
	case models.OperatorEQ:
		return value == condition.Value
	case models.OperatorNEQ:
		return value != condition.Value
	case models.OperatorIn:
		return contains(
			condition.Values,
			value,
		)
	case models.OperatorNotIn:
		return !contains(
			condition.Values,
			value,
		)
	case models.OperatorContains:
		return strings.Contains(
			value,
			condition.Value,
		)
	case models.OperatorNotContains:
		return !strings.Contains(
			value,
			condition.Value,
		)
	case models.OperatorStartsWith:
		return strings.HasPrefix(
			value,
			condition.Value,
		)
	case models.OperatorEndsWith:
		return strings.HasSuffix(
			value,
			condition.Value,
		)
	case models.OperatorGT, models.OperatorGTE, models.OperatorLT, models.OperatorLTE:
		return compareNumeric(
			value,
			condition.Value,
			condition.Operator,
		)
	case models.OperatorRegex:
		return matchRegex(
			value,
			condition.Value,
		)
	default:
		return false
	}
}

func contains(
	values []string,
	target string,
) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func compareNumeric(
	leftRaw,
	rightRaw string,
	op models.ConditionOperator,
) bool {
	left, err := strconv.ParseFloat(
		leftRaw,
		64,
	)
	if err != nil {
		return false
	}
	right, err := strconv.ParseFloat(
		rightRaw,
		64,
	)
	if err != nil {
		return false
	}

	switch op {
	case models.OperatorGT:
		return left > right
	case models.OperatorGTE:
		return left >= right
	case models.OperatorLT:
		return left < right
	case models.OperatorLTE:
		return left <= right
	default:
		return false
	}
}

func matchRegex(
	value,
	pattern string,
) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(value)
}
