package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type captureDelivery struct {
	email string
	otp   string
}

func (d *captureDelivery) SendOTP(
	ctx context.Context,
	email,
	otp string,
) error {
	d.email = email
	d.otp = otp
	return nil
}

func TestRequestOTPSendsDelivery(t *testing.T) {
	delivery := &captureDelivery{}
	service := NewServiceWithDelivery(
		NewMemoryRepository(),
		delivery,
		testConfig(),
	)

	challenge, err := service.RequestOTP(
		context.Background(),
		"admin@example.com",
	)
	if err != nil {
		t.Fatalf(
			"request otp: %v",
			err,
		)
	}
	if delivery.email != "admin@example.com" {
		t.Fatalf(
			"delivery email = %q, want admin@example.com",
			delivery.email,
		)
	}
	if delivery.otp == "" {
		t.Fatal("delivery otp is empty")
	}
	if challenge.OTP == "" {
		t.Fatal("dev challenge otp is empty")
	}
}

func TestResendDeliverySendsOTP(t *testing.T) {
	var request struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		if r.Header.Get("Authorization") != "Bearer re_test" {
			t.Fatalf(
				"Authorization = %q",
				r.Header.Get("Authorization"),
			)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf(
				"decode request: %v",
				err,
			)
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"id":"email_123"}`)); err != nil {
			t.Fatalf(
				"write response: %v",
				err,
			)
		}
	}))
	defer server.Close()

	delivery := NewResendDeliveryWithClient(
		server.Client(),
		ResendConfig{
			APIKey:      "re_test",
			APIURL:      server.URL,
			From:        "TraffoFlex <login@example.com>",
			MaxAttempts: 1,
		},
	)
	if err := delivery.SendOTP(
		context.Background(),
		"user@example.com",
		"123456",
	); err != nil {
		t.Fatalf(
			"send otp: %v",
			err,
		)
	}
	if request.From != "TraffoFlex <login@example.com>" {
		t.Fatalf(
			"from = %q",
			request.From,
		)
	}
	if len(request.To) != 1 || request.To[0] != "user@example.com" {
		t.Fatalf(
			"to = %#v",
			request.To,
		)
	}
	if request.Subject == "" || request.Text == "" {
		t.Fatalf(
			"request = %#v, want subject and text",
			request,
		)
	}
}

func TestResendDeliveryRetriesRetryableFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			if _, err := w.Write([]byte(`{"message":"rate limited"}`)); err != nil {
				t.Fatalf(
					"write rate limit response: %v",
					err,
				)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"id":"email_retry"}`)); err != nil {
			t.Fatalf(
				"write success response: %v",
				err,
			)
		}
	}))
	defer server.Close()

	delivery := NewResendDeliveryWithClient(
		server.Client(),
		ResendConfig{
			APIKey:       "re_test",
			APIURL:       server.URL,
			From:         "TraffoFlex <login@example.com>",
			MaxAttempts:  2,
			RetryBackoff: time.Millisecond,
		},
	)
	if err := delivery.SendOTP(
		context.Background(),
		"user@example.com",
		"123456",
	); err != nil {
		t.Fatalf(
			"send otp: %v",
			err,
		)
	}
	if attempts != 2 {
		t.Fatalf(
			"attempts = %d, want 2",
			attempts,
		)
	}
}

func TestResendDeliveryDoesNotRetryPermanentFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
		if _, err := w.Write([]byte(`{"message":"bad key"}`)); err != nil {
			t.Fatalf(
				"write response: %v",
				err,
			)
		}
	}))
	defer server.Close()

	delivery := NewResendDeliveryWithClient(
		server.Client(),
		ResendConfig{
			APIKey:       "re_test",
			APIURL:       server.URL,
			From:         "TraffoFlex <login@example.com>",
			MaxAttempts:  3,
			RetryBackoff: time.Millisecond,
		},
	)
	if err := delivery.SendOTP(
		context.Background(),
		"user@example.com",
		"123456",
	); err == nil {
		t.Fatal("expected send error")
	}
	if attempts != 1 {
		t.Fatalf(
			"attempts = %d, want 1",
			attempts,
		)
	}
}
