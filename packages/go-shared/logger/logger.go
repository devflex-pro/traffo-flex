package logger

import (
	"log/slog"
	"os"
)

func New(service string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{},
	)).With(
		"service",
		service,
	)
}
