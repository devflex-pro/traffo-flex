package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAdminOTPCreatesSession(t *testing.T) {
	service := newTestService()

	challenge, err := service.RequestOTP(
		context.Background(),
		"Admin@Example.com",
	)
	if err != nil {
		t.Fatalf(
			"request otp: %v",
			err,
		)
	}
	if challenge.OTP == "" {
		t.Fatal("otp is empty")
	}

	session, err := service.VerifyOTP(
		context.Background(),
		"admin@example.com",
		challenge.OTP,
	)
	if err != nil {
		t.Fatalf(
			"verify otp: %v",
			err,
		)
	}
	if session.Token == "" {
		t.Fatal("token is empty")
	}
	if session.User.Role != RoleAdmin || !session.User.Approved {
		t.Fatalf(
			"user = %+v, want approved admin",
			session.User,
		)
	}

	user, err := service.Authenticate(
		context.Background(),
		session.Token,
	)
	if err != nil {
		t.Fatalf(
			"authenticate: %v",
			err,
		)
	}
	if user.Email != "admin@example.com" {
		t.Fatalf(
			"email = %q, want admin@example.com",
			user.Email,
		)
	}
}

func TestNonAdminRequiresApprovalAfterOTP(t *testing.T) {
	service := newTestService()

	challenge, err := service.RequestOTP(
		context.Background(),
		"user@example.com",
	)
	if err != nil {
		t.Fatalf(
			"request otp: %v",
			err,
		)
	}

	session, err := service.VerifyOTP(
		context.Background(),
		"user@example.com",
		challenge.OTP,
	)
	if !errors.Is(
		err,
		ErrPendingApproval,
	) {
		t.Fatalf(
			"verify err = %v, want pending approval",
			err,
		)
	}
	if session.Token != "" {
		t.Fatal("pending user received token")
	}
	if session.User.Status != StatusPendingApproval {
		t.Fatalf(
			"status = %q, want pending approval",
			session.User.Status,
		)
	}

	approved, err := service.ApproveUser(
		context.Background(),
		session.User.ID,
	)
	if err != nil {
		t.Fatalf(
			"approve user: %v",
			err,
		)
	}
	if !approved.Approved || approved.Status != StatusActive {
		t.Fatalf(
			"approved = %+v, want active approved",
			approved,
		)
	}
}

func TestInvalidOTPRejected(t *testing.T) {
	service := newTestService()
	_, err := service.RequestOTP(
		context.Background(),
		"user@example.com",
	)
	if err != nil {
		t.Fatalf(
			"request otp: %v",
			err,
		)
	}

	_, err = service.VerifyOTP(
		context.Background(),
		"user@example.com",
		"000000",
	)
	if !errors.Is(
		err,
		ErrInvalidOTP,
	) {
		t.Fatalf(
			"verify err = %v, want invalid otp",
			err,
		)
	}
}

func TestOTPAttemptsLimitedAndResetWithNewCode(t *testing.T) {
	service := newTestService()
	now := time.Date(
		2026,
		10,
		4,
		12,
		0,
		0,
		0,
		time.UTC,
	)
	service.now = func() time.Time { return now }
	challenge, err := service.RequestOTP(
		context.Background(),
		"admin@example.com",
	)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < maxOTPAttempts; attempt++ {
		_, err = service.VerifyOTP(
			context.Background(),
			"admin@example.com",
			"invalid",
		)
		if !errors.Is(err, ErrInvalidOTP) {
			t.Fatalf(
				"attempt %d: %v",
				attempt,
				err,
			)
		}
	}
	_, err = service.VerifyOTP(
		context.Background(),
		"admin@example.com",
		challenge.OTP,
	)
	if !errors.Is(err, ErrOTPAttempts) {
		t.Fatalf(
			"correct code after lockout = %v",
			err,
		)
	}
	now = now.Add(13 * time.Minute)
	challenge, err = service.RequestOTP(
		context.Background(),
		"admin@example.com",
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.VerifyOTP(
		context.Background(),
		"admin@example.com",
		challenge.OTP,
	); err != nil {
		t.Fatal(err)
	}
}

func TestLogoutRevokesAllUserSessions(t *testing.T) {
	service := newTestService()
	challenge, err := service.RequestOTP(
		context.Background(),
		"admin@example.com",
	)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.VerifyOTP(
		context.Background(),
		"admin@example.com",
		challenge.OTP,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(
		context.Background(),
		session.Token,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(
		context.Background(),
		session.Token,
	); !errors.Is(
		err,
		ErrInvalidToken,
	) {
		t.Fatalf(
			"revoked token = %v",
			err,
		)
	}
}

func TestRequestOTPRejectsActiveOTP(t *testing.T) {
	service := newTestService()
	_, err := service.RequestOTP(
		context.Background(),
		"user@example.com",
	)
	if err != nil {
		t.Fatalf(
			"request otp: %v",
			err,
		)
	}

	_, err = service.RequestOTP(
		context.Background(),
		"user@example.com",
	)
	if !errors.Is(
		err,
		ErrOTPActive,
	) {
		t.Fatalf(
			"request err = %v, want active otp",
			err,
		)
	}
}

func TestRequestOTPRateLimitedAfterOTPExpires(t *testing.T) {
	service := newTestService()
	now := time.Date(
		2026,
		5,
		16,
		12,
		0,
		0,
		0,
		time.UTC,
	)
	service.now = func() time.Time {
		return now
	}
	_, err := service.RequestOTP(
		context.Background(),
		"user@example.com",
	)
	if err != nil {
		t.Fatalf(
			"request otp: %v",
			err,
		)
	}

	now = now.Add(11 * time.Minute)
	_, err = service.RequestOTP(
		context.Background(),
		"user@example.com",
	)
	if !errors.Is(
		err,
		ErrOTPRateLimited,
	) {
		t.Fatalf(
			"request err = %v, want rate limited",
			err,
		)
	}

	now = now.Add(time.Minute)
	challenge, err := service.RequestOTP(
		context.Background(),
		"user@example.com",
	)
	if err != nil {
		t.Fatalf(
			"request after rate limit: %v",
			err,
		)
	}
	if challenge.OTP == "" {
		t.Fatal("otp is empty")
	}
}

func newTestService() *Service {
	return NewService(
		NewMemoryRepository(),
		testConfig(),
	)
}

func testConfig() Config {
	return Config{
		AdminEmail:   "admin@example.com",
		JWTSecret:    "test-secret",
		OTPTTL:       10 * time.Minute,
		OTPRateLimit: 12 * time.Minute,
		SessionTTL:   time.Hour,
		DevReturnOTP: true,
	}
}
