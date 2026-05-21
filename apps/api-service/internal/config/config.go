package config

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	sharedconfig "github.com/devflex/traffoflex/packages/go-shared/config"
)

type Config struct {
	Addr                   string
	MongoURI               string
	MongoDatabase          string
	ClickHouseHTTPURL      string
	TrafficServiceURL      string
	PostbackServiceURL     string
	AdminFrontendOrigin    string
	AuthAdminEmail         string
	AuthJWTSecret          string
	AuthOTPTTLSeconds      int
	AuthOTPRateLimit       int
	AuthSessionTTL         int
	AuthEnv                string
	AuthDevReturnOTP       bool
	AuthEmailFrom          string
	AuthResendAPIKey       string
	AuthResendAPIURL       string
	AuthResendMaxAttempts  int
	AuthResendRetryBackoff int
}

func Load() Config {
	return Config{
		Addr: sharedconfig.Env(
			"API_SERVICE_ADDR",
			":8070",
		),
		MongoURI: sharedconfig.Env(
			"MONGO_URI",
			"mongodb://mongo:27017",
		),
		MongoDatabase: sharedconfig.Env(
			"MONGO_DATABASE",
			"traffoflex",
		),
		ClickHouseHTTPURL: sharedconfig.Env(
			"CLICKHOUSE_HTTP_URL",
			"http://default:traffoflex@clickhouse:8123?database=traffoflex",
		),
		TrafficServiceURL: sharedconfig.Env(
			"TRAFFIC_SERVICE_URL",
			"http://traffic-service:8080",
		),
		PostbackServiceURL: sharedconfig.Env(
			"POSTBACK_SERVICE_URL",
			"http://postback-service:8081",
		),
		AdminFrontendOrigin: sharedconfig.Env(
			"ADMIN_FRONTEND_ORIGIN",
			"http://localhost:5173",
		),
		AuthAdminEmail: sharedconfig.Env(
			"AUTH_ADMIN_EMAIL",
			"admin@example.com",
		),
		AuthJWTSecret: sharedconfig.Env(
			"AUTH_JWT_SECRET",
			"change-me-local-dev-secret",
		),
		AuthOTPTTLSeconds: envIntFallbackMinutesAsSeconds(
			"AUTH_OTP_TTL_SECONDS",
			"AUTH_OTP_TTL_MINUTES",
			10,
		),
		AuthOTPRateLimit: sharedconfig.EnvInt(
			"AUTH_OTP_RATE_LIMIT_MINUTES",
			1,
		),
		AuthSessionTTL: sharedconfig.EnvInt(
			"AUTH_SESSION_TTL_HOURS",
			24,
		),
		AuthEnv: sharedconfig.Env(
			"AUTH_ENV",
			"local",
		),
		AuthDevReturnOTP: envBool(
			"AUTH_DEV_RETURN_OTP",
			false,
		),
		AuthEmailFrom: sharedconfig.Env(
			"AUTH_EMAIL_FROM",
			"noreply@example.com",
		),
		AuthResendAPIKey: sharedconfig.Env(
			"AUTH_RESEND_API_KEY",
			"",
		),
		AuthResendAPIURL: sharedconfig.Env(
			"AUTH_RESEND_API_URL",
			"https://api.resend.com/emails",
		),
		AuthResendMaxAttempts: sharedconfig.EnvInt(
			"AUTH_RESEND_MAX_ATTEMPTS",
			3,
		),
		AuthResendRetryBackoff: sharedconfig.EnvInt(
			"AUTH_RESEND_RETRY_BACKOFF_MS",
			500,
		),
	}
}

func envBool(
	key string,
	fallback bool,
) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes"
}

func (c Config) Validate() error {
	return errors.Join(
		sharedconfig.RequireNonEmpty(
			"API_SERVICE_ADDR",
			c.Addr,
		),
		sharedconfig.RequireURL(
			"MONGO_URI",
			c.MongoURI,
			"mongodb",
			"mongodb+srv",
		),
		sharedconfig.RequireNonEmpty(
			"MONGO_DATABASE",
			c.MongoDatabase,
		),
		sharedconfig.RequireURL(
			"CLICKHOUSE_HTTP_URL",
			c.ClickHouseHTTPURL,
			"http",
			"https",
		),
		sharedconfig.RequireURL(
			"TRAFFIC_SERVICE_URL",
			c.TrafficServiceURL,
			"http",
			"https",
		),
		sharedconfig.RequireURL(
			"POSTBACK_SERVICE_URL",
			c.PostbackServiceURL,
			"http",
			"https",
		),
		sharedconfig.RequireNonEmpty(
			"AUTH_ADMIN_EMAIL",
			c.AuthAdminEmail,
		),
		sharedconfig.RequireNonEmpty(
			"AUTH_JWT_SECRET",
			c.AuthJWTSecret,
		),
		sharedconfig.RequirePositive(
			"AUTH_OTP_TTL_SECONDS",
			c.AuthOTPTTLSeconds,
		),
		sharedconfig.RequirePositive(
			"AUTH_OTP_RATE_LIMIT_MINUTES",
			c.AuthOTPRateLimit,
		),
		sharedconfig.RequirePositive(
			"AUTH_SESSION_TTL_HOURS",
			c.AuthSessionTTL,
		),
		sharedconfig.RequirePositive(
			"AUTH_RESEND_MAX_ATTEMPTS",
			c.AuthResendMaxAttempts,
		),
		sharedconfig.RequirePositive(
			"AUTH_RESEND_RETRY_BACKOFF_MS",
			c.AuthResendRetryBackoff,
		),
		validateAuthDelivery(c),
	)
}

func envIntFallbackMinutesAsSeconds(
	secondsKey string,
	minutesKey string,
	fallbackMinutes int,
) int {
	if raw, ok := os.LookupEnv(secondsKey); ok {
		parsed, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return 0
		}
		return parsed
	}
	return sharedconfig.EnvInt(
		minutesKey,
		fallbackMinutes,
	) * 60
}

func (c Config) Ready(r *http.Request) error {
	return c.Validate()
}

func (c Config) LocalAuthEnv() bool {
	env := strings.ToLower(strings.TrimSpace(c.AuthEnv))
	return env == "local" || env == "dev" || env == "test"
}

func (c Config) ResendConfigured() bool {
	return strings.TrimSpace(c.AuthResendAPIKey) != ""
}

func validateAuthDelivery(c Config) error {
	if c.AuthDevReturnOTP && !c.LocalAuthEnv() {
		return errors.New("AUTH_DEV_RETURN_OTP is allowed only for AUTH_ENV local, dev or test")
	}
	if c.LocalAuthEnv() {
		return nil
	}
	if strings.TrimSpace(c.AuthEmailFrom) == "" {
		return errors.New("AUTH_EMAIL_FROM is required outside local auth env")
	}
	if strings.TrimSpace(c.AuthResendAPIKey) == "" {
		return errors.New("AUTH_RESEND_API_KEY is required outside local auth env")
	}
	if strings.TrimSpace(c.AuthResendAPIURL) == "" {
		return errors.New("AUTH_RESEND_API_URL is required outside local auth env")
	}
	return nil
}
