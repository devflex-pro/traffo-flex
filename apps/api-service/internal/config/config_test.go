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

func TestTrackerPublicURLValidation(t *testing.T) {
	cfg := Load()
	cfg.TrackerPublicURL = "https://go.example.com/path"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected tracker URL with a path to be rejected")
	}
	cfg.TrackerPublicURL = "http://go.example.com"
	cfg.AuthEnv = "production"
	cfg.AuthResendAPIKey = "re_test"
	cfg.AuthEmailFrom = "noreply@example.com"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected insecure production tracker URL to be rejected")
	}
	cfg.TrackerPublicURL = "https://go.example.com"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid production tracker URL rejected: %v", err)
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

func TestValidateRejectsDevOTPOutsideLocalEnv(t *testing.T) {
	cfg := Load()
	cfg.AuthEnv = "production"
	cfg.AuthDevReturnOTP = true
	cfg.AuthResendAPIKey = "re_test"
	cfg.AuthEmailFrom = "noreply@example.com"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateRequiresResendOutsideLocalEnv(t *testing.T) {
	cfg := Load()
	cfg.AuthEnv = "production"
	cfg.AuthDevReturnOTP = false
	cfg.AuthResendAPIKey = ""

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv(
		"API_SERVICE_ADDR",
		":9000",
	)
	t.Setenv(
		"MONGO_URI",
		"mongodb://localhost:27017",
	)
	t.Setenv(
		"MONGO_DATABASE",
		"testdb",
	)
	t.Setenv(
		"CLICKHOUSE_HTTP_URL",
		"http://localhost:8123",
	)
	t.Setenv(
		"TRAFFIC_SERVICE_URL",
		"http://localhost:8080",
	)
	t.Setenv(
		"POSTBACK_SERVICE_URL",
		"http://localhost:8081",
	)
	t.Setenv(
		"ADMIN_FRONTEND_ORIGIN",
		"http://localhost:5173",
	)
	t.Setenv(
		"AUTH_ENV",
		"production",
	)
	t.Setenv(
		"AUTH_RESEND_API_KEY",
		"re_test",
	)
	t.Setenv(
		"AUTH_EMAIL_FROM",
		"noreply@example.com",
	)

	cfg := Load()

	if cfg.Addr != ":9000" {
		t.Fatalf(
			"Addr = %q, want :9000",
			cfg.Addr,
		)
	}
	if cfg.MongoDatabase != "testdb" {
		t.Fatalf(
			"MongoDatabase = %q, want testdb",
			cfg.MongoDatabase,
		)
	}
	if cfg.AuthEnv != "production" {
		t.Fatalf(
			"AuthEnv = %q, want production",
			cfg.AuthEnv,
		)
	}
}
