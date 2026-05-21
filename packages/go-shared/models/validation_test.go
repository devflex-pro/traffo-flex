package models

import (
	"testing"
	"time"
)

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		wantErr bool
	}{
		{name: "valid", slug: "demo-campaign-1"},
		{name: "too short", slug: "ab", wantErr: true},
		{name: "uppercase", slug: "Demo", wantErr: true},
		{name: "leading hyphen", slug: "-demo", wantErr: true},
		{name: "trailing hyphen", slug: "demo-", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				err := ValidateSlug(tt.slug)
				if tt.wantErr && err == nil {
					t.Fatal("expected error")
				}
				if !tt.wantErr && err != nil {
					t.Fatalf(
						"unexpected error: %v",
						err,
					)
				}
			},
		)
	}
}

func TestValidateDestinationURLRejectsUnsafeOverrides(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{name: "valid", rawURL: "https://example.com/path?subid={click_id}"},
		{name: "redirect override", rawURL: "https://example.com/path?redirect_url=https://evil.test", wantErr: true},
		{name: "url override", rawURL: "https://example.com/path?url=https://evil.test", wantErr: true},
		{name: "unsupported scheme", rawURL: "javascript:alert(1)", wantErr: true},
		{name: "missing host", rawURL: "https:///path", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				err := ValidateDestinationURL(tt.rawURL)
				if tt.wantErr && err == nil {
					t.Fatal("expected error")
				}
				if !tt.wantErr && err != nil {
					t.Fatalf(
						"unexpected error: %v",
						err,
					)
				}
			},
		)
	}
}

func TestDestinationAvailable(t *testing.T) {
	destination := Destination{ManualStatus: StatusActive, HealthStatus: HealthHealthy}
	if !destination.Available() {
		t.Fatal("active healthy destination should be available")
	}

	destination.HealthStatus = HealthUnhealthy
	if destination.Available() {
		t.Fatal("unhealthy destination should not be available")
	}
}

func TestDestinationScheduleSameDayWindow(t *testing.T) {
	now := time.Date(
		2026,
		5,
		18,
		10,
		30,
		0,
		0,
		time.UTC,
	)
	destination := Destination{
		ManualStatus: StatusActive,
		HealthStatus: HealthHealthy,
		Schedule: DestinationSchedule{
			Enabled:  true,
			Timezone: "UTC",
			Windows: []ScheduleWindow{
				{
					Weekdays:  []Weekday{WeekdayMonday},
					StartTime: "09:00",
					EndTime:   "18:00",
				},
			},
		},
	}
	if !destination.AvailableAt(now) {
		t.Fatal("destination should be available inside schedule window")
	}
	if destination.AvailableAt(now.Add(10 * time.Hour)) {
		t.Fatal("destination should not be available outside schedule window")
	}
}

func TestDestinationScheduleOvernightWindow(t *testing.T) {
	destination := Destination{
		ManualStatus: StatusActive,
		HealthStatus: HealthHealthy,
		Schedule: DestinationSchedule{
			Enabled:  true,
			Timezone: "UTC",
			Windows: []ScheduleWindow{
				{
					Weekdays:  []Weekday{WeekdayMonday},
					StartTime: "22:00",
					EndTime:   "02:00",
				},
			},
		},
	}
	tuesdayNight := time.Date(
		2026,
		5,
		19,
		1,
		30,
		0,
		0,
		time.UTC,
	)
	if !destination.AvailableAt(tuesdayNight) {
		t.Fatal("destination should be available in overnight schedule window")
	}
	tuesdayDay := time.Date(
		2026,
		5,
		19,
		12,
		0,
		0,
		0,
		time.UTC,
	)
	if destination.AvailableAt(tuesdayDay) {
		t.Fatal("destination should not be available after overnight window")
	}
}

func TestValidateDestinationSchedule(t *testing.T) {
	valid := DestinationSchedule{
		Enabled:  true,
		Timezone: "Europe/Moscow",
		Windows: []ScheduleWindow{
			{
				Weekdays:  []Weekday{WeekdayMonday},
				StartTime: "09:00",
				EndTime:   "18:00",
			},
		},
	}
	if err := ValidateDestinationSchedule(valid); err != nil {
		t.Fatalf(
			"valid schedule rejected: %v",
			err,
		)
	}
	valid.Windows[0].EndTime = "09:00"
	if err := ValidateDestinationSchedule(valid); err == nil {
		t.Fatal("expected equal clock values to be rejected")
	}
}

func TestValidateDestinationCaps(t *testing.T) {
	valid := DestinationCaps{
		Enabled: true,
		Rules: []DestinationCapRule{
			{
				Metric:      CapMetricRevenue,
				WindowHours: 24,
				Limit:       1000,
			},
		},
	}
	if err := ValidateDestinationCaps(valid); err != nil {
		t.Fatalf(
			"valid caps rejected: %v",
			err,
		)
	}
	valid.Rules[0].Limit = 0
	if err := ValidateDestinationCaps(valid); err == nil {
		t.Fatal("expected zero cap limit to be rejected")
	}
}

func TestValidateDistribution(t *testing.T) {
	valid := Distribution{
		Mode: DistributionWeighted,
		Destinations: []WeightedTarget{
			{DestinationID: "dst_1", Weight: 10},
			{DestinationID: "dst_2", Weight: 20},
		},
	}
	if err := ValidateDistribution(valid); err != nil {
		t.Fatalf(
			"valid distribution rejected: %v",
			err,
		)
	}

	valid.Destinations[0].Weight = 0
	if err := ValidateDistribution(valid); err == nil {
		t.Fatal("expected invalid weight error")
	}
}

func TestValidateUniqueDestinationPolicy(t *testing.T) {
	policy := UniqueDestinationPolicy{
		Enabled:            true,
		UserKey:            "source_click_id",
		HistoryWindowHours: 168,
		ExhaustedMode:      UniqueExhaustedAllowRepeat,
		SelectionStrategy:  DistributionBestROI,
		ROIWindowHours:     24,
		MinClicks:          100,
		FallbackStrategy:   DistributionRoundRobin,
	}
	if err := ValidateUniqueDestinationPolicy(policy); err != nil {
		t.Fatalf(
			"valid unique policy rejected: %v",
			err,
		)
	}
	policy.UserKey = "bad key"
	if err := ValidateUniqueDestinationPolicy(policy); err == nil {
		t.Fatal("expected invalid user key error")
	}
}

func TestStatusValidation(t *testing.T) {
	if err := ValidateStatus(StatusActive); err != nil {
		t.Fatalf(
			"active status rejected: %v",
			err,
		)
	}
	if err := ValidateStatus(Status("deleted")); err == nil {
		t.Fatal("expected invalid status error")
	}
}
