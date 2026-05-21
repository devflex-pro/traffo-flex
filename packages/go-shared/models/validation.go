package models

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	slugPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)
	publicIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{6,128}$`)
	macroPattern    = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	userKeyPattern  = regexp.MustCompile(`^(source_click_id|user_agent|sub([1-9]|10)|utm_(source|medium|campaign|content|term)|query\.[A-Za-z0-9_.:-]{1,128})$`)
)

func ValidateSlug(slug string) error {
	if !slugPattern.MatchString(slug) {
		return errors.New("slug must be 3-64 chars and contain only lowercase letters, numbers, and hyphens")
	}
	return nil
}

func ValidatePublicID(id string) error {
	if !publicIDPattern.MatchString(id) {
		return errors.New("public id must be 6-128 chars and contain only letters, numbers, underscores, and hyphens")
	}
	return nil
}

func ValidateMacroName(name string) error {
	if !macroPattern.MatchString(name) {
		return errors.New("macro name must start with a lowercase letter and contain only lowercase letters, numbers, and underscores")
	}
	return nil
}

func ValidateDestinationURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return errors.New("destination url must be valid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("destination url must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("destination url must include host")
	}
	if hasUnsafeInboundOverride(parsed.Query()) {
		return errors.New("destination url must not include inbound redirect override parameters")
	}
	return nil
}

func ValidateWeight(weight int) error {
	if weight <= 0 {
		return errors.New("weight must be greater than zero")
	}
	if weight > 1000000 {
		return errors.New("weight is too large")
	}
	return nil
}

func ValidateStatus(status Status) error {
	if !status.Valid() {
		return errors.New("invalid status")
	}
	return nil
}

func ValidateHealthStatus(status HealthStatus) error {
	if !status.Valid() {
		return errors.New("invalid health status")
	}
	return nil
}

func ValidateCurrency(currency string) error {
	if !currencyPattern.MatchString(strings.TrimSpace(currency)) {
		return errors.New("currency must be a 3-letter ISO code")
	}
	return nil
}

func ValidateDistribution(distribution Distribution) error {
	if !distribution.Mode.Valid() {
		return errors.New("invalid distribution mode")
	}
	if len(distribution.Destinations) == 0 {
		return errors.New("distribution requires at least one destination")
	}
	for _, destination := range distribution.Destinations {
		if strings.TrimSpace(destination.DestinationID) == "" {
			return errors.New("distribution destination id is required")
		}
		if err := ValidateWeight(destination.Weight); err != nil {
			return err
		}
	}
	if err := ValidateUniqueDestinationPolicy(distribution.UniquePolicy); err != nil {
		return err
	}
	return nil
}

func ValidateUniqueDestinationPolicy(policy UniqueDestinationPolicy) error {
	if !policy.Enabled {
		return nil
	}
	if !userKeyPattern.MatchString(strings.TrimSpace(policy.UserKey)) {
		return errors.New("unique policy user_key is invalid")
	}
	if policy.HistoryWindowHours <= 0 {
		return errors.New("unique policy history_window_hours must be positive")
	}
	if policy.HistoryWindowHours > 24*366 {
		return errors.New("unique policy history_window_hours is too large")
	}
	if policy.ExhaustedMode == "" {
		policy.ExhaustedMode = UniqueExhaustedAllowRepeat
	}
	if !policy.ExhaustedMode.Valid() {
		return errors.New("unique policy exhausted_mode is invalid")
	}
	if policy.SelectionStrategy == "" {
		policy.SelectionStrategy = DistributionWaterfall
	}
	if !policy.SelectionStrategy.Valid() {
		return errors.New("unique policy selection_strategy is invalid")
	}
	if policy.SelectionStrategy == DistributionBestROI {
		if policy.ROIWindowHours <= 0 {
			return errors.New("unique policy roi_window_hours must be positive")
		}
		if policy.ROIWindowHours > 24*366 {
			return errors.New("unique policy roi_window_hours is too large")
		}
		if policy.MinClicks < 0 {
			return errors.New("unique policy min_clicks must be non-negative")
		}
		if policy.FallbackStrategy == "" {
			policy.FallbackStrategy = DistributionRoundRobin
		}
		if !policy.FallbackStrategy.Valid() || policy.FallbackStrategy == DistributionBestROI {
			return errors.New("unique policy fallback_strategy is invalid")
		}
	}
	return nil
}

func ValidateDestinationSchedule(schedule DestinationSchedule) error {
	if !schedule.Enabled {
		return nil
	}
	if strings.TrimSpace(schedule.Timezone) == "" {
		return errors.New("schedule timezone is required")
	}
	if _, err := time.LoadLocation(schedule.Timezone); err != nil {
		return errors.New("schedule timezone is invalid")
	}
	if len(schedule.Windows) == 0 {
		return errors.New("schedule requires at least one window")
	}
	for _, window := range schedule.Windows {
		if err := ValidateScheduleWindow(window); err != nil {
			return err
		}
	}
	return nil
}

func ValidateScheduleWindow(window ScheduleWindow) error {
	if len(window.Weekdays) == 0 {
		return errors.New("schedule window requires weekdays")
	}
	seen := make(map[Weekday]struct{})
	for _, weekday := range window.Weekdays {
		if !weekday.Valid() {
			return errors.New("schedule window contains invalid weekday")
		}
		if _, exists := seen[weekday]; exists {
			return errors.New("schedule window contains duplicate weekday")
		}
		seen[weekday] = struct{}{}
	}
	start, err := parseClock(window.StartTime)
	if err != nil {
		return errors.New("schedule window start_time must use HH:MM")
	}
	end, err := parseClock(window.EndTime)
	if err != nil {
		return errors.New("schedule window end_time must use HH:MM")
	}
	if start == end {
		return errors.New("schedule window start_time and end_time must differ")
	}
	return nil
}

func ValidateDestinationCaps(caps DestinationCaps) error {
	if !caps.Enabled {
		return nil
	}
	if len(caps.Rules) == 0 {
		return errors.New("caps require at least one rule")
	}
	for _, rule := range caps.Rules {
		if err := ValidateDestinationCapRule(rule); err != nil {
			return err
		}
	}
	return nil
}

func ValidateDestinationCapRule(rule DestinationCapRule) error {
	if !rule.Metric.Valid() {
		return errors.New("cap rule metric is invalid")
	}
	if rule.WindowHours <= 0 {
		return errors.New("cap rule window_hours must be positive")
	}
	if rule.WindowHours > 24*366 {
		return errors.New("cap rule window_hours is too large")
	}
	if rule.Limit <= 0 {
		return errors.New("cap rule limit must be positive")
	}
	return nil
}

func hasUnsafeInboundOverride(values url.Values) bool {
	for key := range values {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "url" || normalized == "redirect_url" {
			return true
		}
	}
	return false
}
