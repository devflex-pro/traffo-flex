package models

import "time"

type Status string

const (
	StatusActive   Status = "active"
	StatusPaused   Status = "paused"
	StatusArchived Status = "archived"
)

func (s Status) Valid() bool {
	switch s {
	case StatusActive, StatusPaused, StatusArchived:
		return true
	default:
		return false
	}
}

type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthUnknown   HealthStatus = "unknown"
)

func (s HealthStatus) Valid() bool {
	switch s {
	case HealthHealthy, HealthDegraded, HealthUnhealthy, HealthUnknown:
		return true
	default:
		return false
	}
}

type DestinationType string

const (
	DestinationURL         DestinationType = "url"
	DestinationLanding     DestinationType = "landing"
	DestinationOffer       DestinationType = "offer"
	DestinationSafePage    DestinationType = "safe_page"
	DestinationTrafficback DestinationType = "trafficback"
	DestinationFallback    DestinationType = "fallback"
)

func (t DestinationType) Valid() bool {
	switch t {
	case DestinationURL, DestinationLanding, DestinationOffer, DestinationSafePage, DestinationTrafficback, DestinationFallback:
		return true
	default:
		return false
	}
}

type RedirectMode string

const (
	RedirectHTTP302      RedirectMode = "http_302"
	RedirectMetaRefresh  RedirectMode = "meta_refresh"
	RedirectJavaScript   RedirectMode = "javascript"
	RedirectInterstitial RedirectMode = "interstitial"
)

func (m RedirectMode) Valid() bool {
	switch m {
	case RedirectHTTP302, RedirectMetaRefresh, RedirectJavaScript, RedirectInterstitial:
		return true
	default:
		return false
	}
}

type DistributionMode string

const (
	DistributionDirect     DistributionMode = "direct"
	DistributionWeighted   DistributionMode = "weighted"
	DistributionFallback   DistributionMode = "fallback"
	DistributionWaterfall  DistributionMode = "waterfall"
	DistributionRoundRobin DistributionMode = "round_robin"
	DistributionBestROI    DistributionMode = "best_roi"
)

func (m DistributionMode) Valid() bool {
	switch m {
	case DistributionDirect, DistributionWeighted, DistributionFallback,
		DistributionWaterfall, DistributionRoundRobin, DistributionBestROI:
		return true
	default:
		return false
	}
}

type UniqueExhaustedMode string

const (
	UniqueExhaustedAllowRepeat   UniqueExhaustedMode = "allow_repeat"
	UniqueExhaustedNoDestination UniqueExhaustedMode = "no_destination"
)

func (m UniqueExhaustedMode) Valid() bool {
	switch m {
	case UniqueExhaustedAllowRepeat, UniqueExhaustedNoDestination:
		return true
	default:
		return false
	}
}

type ConditionOperator string

const (
	OperatorEQ          ConditionOperator = "eq"
	OperatorNEQ         ConditionOperator = "neq"
	OperatorIn          ConditionOperator = "in"
	OperatorNotIn       ConditionOperator = "not_in"
	OperatorContains    ConditionOperator = "contains"
	OperatorNotContains ConditionOperator = "not_contains"
	OperatorStartsWith  ConditionOperator = "starts_with"
	OperatorEndsWith    ConditionOperator = "ends_with"
	OperatorGT          ConditionOperator = "gt"
	OperatorGTE         ConditionOperator = "gte"
	OperatorLT          ConditionOperator = "lt"
	OperatorLTE         ConditionOperator = "lte"
	OperatorExists      ConditionOperator = "exists"
	OperatorNotExists   ConditionOperator = "not_exists"
	OperatorRegex       ConditionOperator = "regex"
)

func (op ConditionOperator) Valid() bool {
	switch op {
	case OperatorEQ, OperatorNEQ, OperatorIn, OperatorNotIn, OperatorContains, OperatorNotContains,
		OperatorStartsWith, OperatorEndsWith, OperatorGT, OperatorGTE, OperatorLT, OperatorLTE,
		OperatorExists, OperatorNotExists, OperatorRegex:
		return true
	default:
		return false
	}
}

type TrafficbackReason string

const (
	TrafficbackNoMatchingStream     TrafficbackReason = "no_matching_stream"
	TrafficbackNoDestination        TrafficbackReason = "no_destination_available"
	TrafficbackDestinationDisabled  TrafficbackReason = "destination_disabled"
	TrafficbackDestinationUnhealthy TrafficbackReason = "destination_unhealthy"
	TrafficbackOfferCapReached      TrafficbackReason = "offer_cap_reached"
	TrafficbackGeoNotAllowed        TrafficbackReason = "geo_not_allowed"
	TrafficbackDeviceNotAllowed     TrafficbackReason = "device_not_allowed"
	TrafficbackBotOrLowQuality      TrafficbackReason = "bot_or_low_quality"
	TrafficbackPartnerRejectedClick TrafficbackReason = "partner_rejected_click"
	TrafficbackPostbackRejected     TrafficbackReason = "postback_rejected"
	TrafficbackTimeoutOrError       TrafficbackReason = "timeout_or_error"
)

type RedirectConfig struct {
	Mode RedirectMode `json:"mode"`
}

type TrafficbackConfig struct {
	Enabled          bool   `json:"enabled"`
	URL              string `json:"url,omitempty"`
	MaxDepth         int    `json:"max_depth"`
	FallbackCampaign string `json:"fallback_campaign,omitempty"`
	FallbackURL      string `json:"fallback_url,omitempty"`
}

type Campaign struct {
	ID                string            `json:"id"`
	OwnerID           string            `json:"owner_id,omitempty"`
	PublicID          string            `json:"public_id,omitempty"`
	PublicToken       string            `json:"public_token,omitempty"`
	TeamID            string            `json:"team_id"`
	Name              string            `json:"name"`
	Slug              string            `json:"slug"`
	Status            Status            `json:"status"`
	EntryURL          string            `json:"entry_url,omitempty"`
	CustomDomain      string            `json:"custom_domain,omitempty"`
	TrafficSourceID   string            `json:"traffic_source_id,omitempty"`
	Currency          string            `json:"currency,omitempty"`
	DefaultAction     string            `json:"default_action,omitempty"`
	TrafficbackConfig TrafficbackConfig `json:"trafficback_config"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type Stream struct {
	ID                string            `json:"id"`
	OwnerID           string            `json:"owner_id,omitempty"`
	CampaignID        string            `json:"campaign_id"`
	Name              string            `json:"name"`
	Priority          int               `json:"priority"`
	Status            Status            `json:"status"`
	Conditions        []Condition       `json:"conditions"`
	Distribution      Distribution      `json:"distribution"`
	TrafficbackConfig TrafficbackConfig `json:"trafficback_config"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type Condition struct {
	Field    string            `json:"field"`
	Operator ConditionOperator `json:"operator"`
	Values   []string          `json:"values,omitempty"`
	Value    string            `json:"value,omitempty"`
}

type Distribution struct {
	Mode         DistributionMode        `json:"mode"`
	Destinations []WeightedTarget        `json:"destinations"`
	UniquePolicy UniqueDestinationPolicy `json:"unique_policy"`
}

type UniqueDestinationPolicy struct {
	Enabled            bool                `json:"enabled"`
	UserKey            string              `json:"user_key,omitempty"`
	HistoryWindowHours int                 `json:"history_window_hours,omitempty"`
	ExhaustedMode      UniqueExhaustedMode `json:"exhausted_mode,omitempty"`
	SelectionStrategy  DistributionMode    `json:"selection_strategy,omitempty"`
	ROIWindowHours     int                 `json:"roi_window_hours,omitempty"`
	MinClicks          int                 `json:"min_clicks,omitempty"`
	FallbackStrategy   DistributionMode    `json:"fallback_strategy,omitempty"`
}

type WeightedTarget struct {
	DestinationID string `json:"destination_id"`
	Weight        int    `json:"weight"`
}

type Weekday string

const (
	WeekdayMonday    Weekday = "mon"
	WeekdayTuesday   Weekday = "tue"
	WeekdayWednesday Weekday = "wed"
	WeekdayThursday  Weekday = "thu"
	WeekdayFriday    Weekday = "fri"
	WeekdaySaturday  Weekday = "sat"
	WeekdaySunday    Weekday = "sun"
)

func (d Weekday) Valid() bool {
	switch d {
	case WeekdayMonday, WeekdayTuesday, WeekdayWednesday, WeekdayThursday,
		WeekdayFriday, WeekdaySaturday, WeekdaySunday:
		return true
	default:
		return false
	}
}

type DestinationSchedule struct {
	Enabled  bool             `json:"enabled"`
	Timezone string           `json:"timezone,omitempty"`
	Windows  []ScheduleWindow `json:"windows,omitempty"`
}

type ScheduleWindow struct {
	Weekdays  []Weekday `json:"weekdays"`
	StartTime string    `json:"start_time"`
	EndTime   string    `json:"end_time"`
}

type CapMetric string

const (
	CapMetricClicks      CapMetric = "clicks"
	CapMetricCost        CapMetric = "cost"
	CapMetricConversions CapMetric = "conversions"
	CapMetricRevenue     CapMetric = "revenue"
)

func (m CapMetric) Valid() bool {
	switch m {
	case CapMetricClicks, CapMetricCost, CapMetricConversions, CapMetricRevenue:
		return true
	default:
		return false
	}
}

type DestinationCaps struct {
	Enabled bool                 `json:"enabled"`
	Rules   []DestinationCapRule `json:"rules,omitempty"`
}

type DestinationCapRule struct {
	Metric      CapMetric `json:"metric"`
	WindowHours int       `json:"window_hours"`
	Limit       float64   `json:"limit"`
}

type Destination struct {
	ID                string              `json:"id"`
	OwnerID           string              `json:"owner_id,omitempty"`
	Name              string              `json:"name"`
	Type              DestinationType     `json:"type"`
	URL               string              `json:"url"`
	HealthcheckURL    string              `json:"healthcheck_url,omitempty"`
	ManualStatus      Status              `json:"manual_status"`
	HealthStatus      HealthStatus        `json:"health_status"`
	Redirect          RedirectConfig      `json:"redirect"`
	TrafficbackConfig TrafficbackConfig   `json:"trafficback_config"`
	Schedule          DestinationSchedule `json:"schedule"`
	Caps              DestinationCaps     `json:"caps"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

func (d Destination) Available() bool {
	return d.AvailableAt(time.Now().UTC())
}

func (d Destination) AvailableAt(now time.Time) bool {
	return d.ManualStatus == StatusActive &&
		d.HealthStatus != HealthUnhealthy &&
		d.Schedule.AvailableAt(now)
}

func (s DestinationSchedule) AvailableAt(now time.Time) bool {
	if !s.Enabled {
		return true
	}
	location := time.UTC
	if s.Timezone != "" {
		loaded, err := time.LoadLocation(s.Timezone)
		if err == nil {
			location = loaded
		}
	}
	local := now.In(location)
	for _, window := range s.Windows {
		if window.Contains(local) {
			return true
		}
	}
	return false
}

func (w ScheduleWindow) Contains(local time.Time) bool {
	start, startErr := parseClock(w.StartTime)
	end, endErr := parseClock(w.EndTime)
	if startErr != nil || endErr != nil || start == end {
		return false
	}
	current := local.Hour()*60 + local.Minute()
	today := weekdayFromTime(local)
	yesterday := weekdayFromTime(local.AddDate(
		0,
		0,
		-1,
	))
	if start < end {
		return hasWeekday(
			w.Weekdays,
			today,
		) && current >= start && current < end
	}
	return (hasWeekday(
		w.Weekdays,
		today,
	) && current >= start) || (hasWeekday(
		w.Weekdays,
		yesterday,
	) && current < end)
}

func parseClock(value string) (
	int,
	error,
) {
	parsed, err := time.Parse(
		"15:04",
		value,
	)
	if err != nil {
		return 0, err
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func weekdayFromTime(value time.Time) Weekday {
	switch value.Weekday() {
	case time.Monday:
		return WeekdayMonday
	case time.Tuesday:
		return WeekdayTuesday
	case time.Wednesday:
		return WeekdayWednesday
	case time.Thursday:
		return WeekdayThursday
	case time.Friday:
		return WeekdayFriday
	case time.Saturday:
		return WeekdaySaturday
	default:
		return WeekdaySunday
	}
}

func hasWeekday(
	weekdays []Weekday,
	weekday Weekday,
) bool {
	for _, item := range weekdays {
		if item == weekday {
			return true
		}
	}
	return false
}

type TrafficSource struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id,omitempty"`
	TeamID    string    `json:"team_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AffiliateNetwork struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id,omitempty"`
	TeamID    string    `json:"team_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PostbackTemplate struct {
	ID        string            `json:"id"`
	OwnerID   string            `json:"owner_id,omitempty"`
	TeamID    string            `json:"team_id"`
	NetworkID string            `json:"network_id"`
	Name      string            `json:"name"`
	Slug      string            `json:"slug"`
	Secret    string            `json:"secret,omitempty"`
	Mapping   map[string]string `json:"mapping"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}
