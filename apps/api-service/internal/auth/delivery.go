package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type OTPDelivery interface {
	SendOTP(
		ctx context.Context,
		email,
		otp string,
	) error
}

type LogDelivery struct {
	log *slog.Logger
}

func NewLogDelivery(log *slog.Logger) *LogDelivery {
	return &LogDelivery{log: log}
}

func (d *LogDelivery) SendOTP(
	ctx context.Context,
	email,
	otp string,
) error {
	d.log.Info(
		"auth otp generated",
		"email",
		email,
		"otp",
		otp,
	)
	return nil
}

type ResendConfig struct {
	APIKey       string
	APIURL       string
	From         string
	MaxAttempts  int
	RetryBackoff time.Duration
	Logger       *slog.Logger
}

type ResendDelivery struct {
	client *http.Client
	cfg    ResendConfig
}

func NewResendDelivery(cfg ResendConfig) *ResendDelivery {
	return NewResendDeliveryWithClient(
		http.DefaultClient,
		cfg,
	)
}

func NewResendDeliveryWithClient(
	client *http.Client,
	cfg ResendConfig,
) *ResendDelivery {
	if client == nil {
		client = http.DefaultClient
	}
	if strings.TrimSpace(cfg.APIURL) == "" {
		cfg.APIURL = "https://api.resend.com/emails"
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = 500 * time.Millisecond
	}
	return &ResendDelivery{
		client: client,
		cfg:    cfg,
	}
}

func (d *ResendDelivery) SendOTP(
	ctx context.Context,
	email,
	otp string,
) error {
	if strings.TrimSpace(d.cfg.APIKey) == "" {
		return errors.New("resend api key is required")
	}
	var lastErr error
	for attempt := 1; attempt <= d.cfg.MaxAttempts; attempt++ {
		messageID, retryable, err := d.sendOnce(
			ctx,
			email,
			otp,
		)
		if err == nil {
			d.logInfo(
				"otp email sent",
				"provider",
				"resend",
				"email",
				email,
				"message_id",
				messageID,
				"attempt",
				attempt,
			)
			return nil
		}
		lastErr = err
		if !retryable || attempt == d.cfg.MaxAttempts {
			d.logWarn(
				"otp email send failed",
				"provider",
				"resend",
				"email",
				email,
				"attempt",
				attempt,
				"max_attempts",
				d.cfg.MaxAttempts,
				"retryable",
				retryable,
				"error",
				err,
			)
			return err
		}
		d.logWarn(
			"otp email send retrying",
			"provider",
			"resend",
			"email",
			email,
			"attempt",
			attempt,
			"next_attempt",
			attempt+1,
			"max_attempts",
			d.cfg.MaxAttempts,
			"retry_backoff_ms",
			d.cfg.RetryBackoff.Milliseconds(),
			"error",
			err,
		)
		timer := time.NewTimer(d.cfg.RetryBackoff)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func (d *ResendDelivery) sendOnce(
	ctx context.Context,
	email,
	otp string,
) (
	string,
	bool,
	error,
) {
	payload := map[string]any{
		"from":    d.cfg.From,
		"to":      []string{email},
		"subject": "TraffoFlex login code",
		"text": fmt.Sprintf(
			"Your TraffoFlex login code is: %s\nThis code expires soon.",
			otp,
		),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", false, err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		d.cfg.APIURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return "", false, err
	}
	req.Header.Set(
		"Authorization",
		"Bearer "+d.cfg.APIKey,
	)
	req.Header.Set(
		"Content-Type",
		"application/json",
	)
	res, err := d.client.Do(req)
	if err != nil {
		return "", true, err
	}
	defer func() {
		_, copyErr := io.Copy(
			io.Discard,
			res.Body,
		)
		closeErr := res.Body.Close()
		if copyErr != nil || closeErr != nil {
			// Response body cleanup errors cannot change the already returned send result.
			return
		}
	}()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		responseBody, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			return "", retryableResendStatus(res.StatusCode), errors.Join(
				fmt.Errorf("resend returned status %d", res.StatusCode),
				readErr,
			)
		}
		return "", retryableResendStatus(res.StatusCode), fmt.Errorf(
			"resend returned status %d: %s",
			res.StatusCode,
			strings.TrimSpace(string(responseBody)),
		)
	}
	var response struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return "", false, err
	}
	return response.ID, false, nil
}

func retryableResendStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func (d *ResendDelivery) logInfo(
	message string,
	args ...any,
) {
	if d.cfg.Logger == nil {
		return
	}
	d.cfg.Logger.Info(
		message,
		args...,
	)
}

func (d *ResendDelivery) logWarn(
	message string,
	args ...any,
) {
	if d.cfg.Logger == nil {
		return
	}
	d.cfg.Logger.Warn(
		message,
		args...,
	)
}
