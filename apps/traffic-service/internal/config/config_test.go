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

func TestTrustedProxyPrefixes(t *testing.T) {
	cfg := Load()
	cfg.TrustedProxyCIDRs = []string{"10.0.0.0/8", "192.168.0.0/16"}

	prefixes, err := cfg.TrustedProxyPrefixes()
	if err != nil {
		t.Fatalf(
			"TrustedProxyPrefixes returned error: %v",
			err,
		)
	}
	if len(prefixes) != 2 {
		t.Fatalf(
			"len(prefixes) = %d, want 2",
			len(prefixes),
		)
	}
}

func TestValidateRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	cfg := Load()
	cfg.TrustedProxyCIDRs = []string{"not-cidr"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}
