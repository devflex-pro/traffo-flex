package postbacklogs

import (
	"context"
	"testing"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestMemoryLogger(t *testing.T) {
	logger := NewMemoryLogger()

	if err := logger.Log(
		context.Background(),
		models.PostbackLogEvent{PostbackID: "pb_1"},
	); err != nil {
		t.Fatalf(
			"Log returned error: %v",
			err,
		)
	}
	if len(logger.Events()) != 1 {
		t.Fatalf(
			"len(events) = %d, want 1",
			len(logger.Events()),
		)
	}
}
