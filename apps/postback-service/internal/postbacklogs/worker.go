package postbacklogs

import (
	"context"
	"log/slog"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type Worker struct {
	repo     DeliveryRepository
	sink     Logger
	log      *slog.Logger
	interval time.Duration
}

type DeliveryRepository interface {
	Pending(
		ctx context.Context,
		limit int,
	) ([]models.PostbackLogEvent, error)
	MarkDelivered(
		ctx context.Context,
		postbackID string,
	) error
}

func NewWorker(
	repo DeliveryRepository,
	sink Logger,
	log *slog.Logger,
	interval time.Duration,
) *Worker {
	if interval <= 0 {
		interval = time.Second
	}
	return &Worker{repo: repo, sink: sink, log: log, interval: interval}
}

func (w *Worker) Run(ctx context.Context) {
	w.RunOnce(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.RunOnce(ctx)
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) {
	events, err := w.repo.Pending(
		ctx,
		100,
	)
	if err != nil {
		w.log.Warn(
			"pending postback logs lookup failed",
			"error",
			err,
		)
		return
	}
	for _, event := range events {
		if err := w.sink.Log(
			ctx,
			event,
		); err != nil {
			w.log.Warn(
				"postback log delivery failed",
				"postback_id",
				event.PostbackID,
				"error",
				err,
			)
			return
		}
		if err := w.repo.MarkDelivered(
			ctx,
			event.PostbackID,
		); err != nil {
			w.log.Warn(
				"postback log delivery checkpoint failed",
				"postback_id",
				event.PostbackID,
				"error",
				err,
			)
			return
		}
	}
}
