package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func Env(
	key,
	fallback string,
) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func EnvInt(
	key string,
	fallback int,
) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func RequirePositive(
	name string,
	value int,
) error {
	if value <= 0 {
		return errors.New(name + " must be positive")
	}
	return nil
}

func RequireNonEmpty(
	name,
	value string,
) error {
	if strings.TrimSpace(value) == "" {
		return errors.New(name + " is required")
	}
	return nil
}

func RequireURL(
	name,
	value string,
	schemes ...string,
) error {
	if err := RequireNonEmpty(
		name,
		value,
	); err != nil {
		return err
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return errors.New(name + " must be a valid URL")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return errors.New(name + " must include scheme and host")
	}
	if len(schemes) == 0 {
		return nil
	}
	for _, scheme := range schemes {
		if parsed.Scheme == scheme {
			return nil
		}
	}
	return errors.New(name + " has unsupported scheme")
}
