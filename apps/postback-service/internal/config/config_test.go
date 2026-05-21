package config

import "testing"

func TestLoadDefaultsAreValid(t *testing.T) {
	cfg := Load()

	if err := cfg.Validate(); err != nil {
		t.Fatalf(
			"default config should be valid: %v",
			err,
		)
	}
}

func TestValidateRejectsInvalidURLs(t *testing.T) {
	cfg := Load()
	cfg.MongoURI = "localhost:27017"
	cfg.ClickHouseHTTPURL = "clickhouse:8123"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
