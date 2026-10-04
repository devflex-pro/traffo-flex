package postbacklogs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type testDeliveryRepo struct {
	event     models.PostbackLogEvent
	delivered bool
}

func (r *testDeliveryRepo) Pending(
	ctx context.Context,
	limit int,
) ([]models.PostbackLogEvent, error) {
	if r.delivered {
		return nil, nil
	}
	return []models.PostbackLogEvent{r.event}, nil
}

func (r *testDeliveryRepo) MarkDelivered(
	ctx context.Context,
	postbackID string,
) error {
	r.delivered = true
	return nil
}

type testDeliverySink struct {
	fail bool
}

func (s *testDeliverySink) Log(
	ctx context.Context,
	event models.PostbackLogEvent,
) error {
	if s.fail {
		return errors.New("broker unavailable")
	}
	return nil
}

func TestWorkerKeepsLogPendingUntilBrokerAck(t *testing.T) {
	repo := &testDeliveryRepo{event: models.PostbackLogEvent{PostbackID: "pb_1"}}
	sink := &testDeliverySink{fail: true}
	worker := NewWorker(
		repo,
		sink,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		time.Second,
	)
	worker.RunOnce(context.Background())
	if repo.delivered {
		t.Fatal("log marked delivered without broker acknowledgement")
	}
	sink.fail = false
	worker.RunOnce(context.Background())
	if !repo.delivered {
		t.Fatal("log still pending after broker acknowledgement")
	}
}
