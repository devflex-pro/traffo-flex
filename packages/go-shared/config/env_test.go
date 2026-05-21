package config

import "testing"

func TestEnvReturnsFallbackForUnsetOrBlankValue(t *testing.T) {
	t.Setenv(
		"TRAFFOFLEX_TEST_ENV",
		"",
	)

	if got := Env(
		"TRAFFOFLEX_TEST_ENV",
		"fallback",
	); got != "fallback" {
		t.Fatalf(
			"Env returned %q, want fallback",
			got,
		)
	}
}

func TestEnvTrimsValue(t *testing.T) {
	t.Setenv(
		"TRAFFOFLEX_TEST_ENV",
		"  value  ",
	)

	if got := Env(
		"TRAFFOFLEX_TEST_ENV",
		"fallback",
	); got != "value" {
		t.Fatalf(
			"Env returned %q, want value",
			got,
		)
	}
}

func TestRequireURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid", value: "http://localhost:8123"},
		{name: "missing scheme", value: "localhost:8123", wantErr: true},
		{name: "unsupported scheme", value: "ftp://localhost", wantErr: true},
		{name: "blank", value: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				err := RequireURL(
					"TEST_URL",
					tt.value,
					"http",
					"https",
				)
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
