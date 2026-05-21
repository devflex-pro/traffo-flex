package healthcheck

import (
	"context"
	"log/slog"
	"time"
)

type Worker struct {
	log      *slog.Logger
	service  *Service
	interval time.Duration
}

func NewWorker(
	log *slog.Logger,
	service *Service,
	interval time.Duration,
) *Worker {
	if interval <= 0 {
		interval = time.Minute
	}
	return &Worker{
		log:      log,
		service:  service,
		interval: interval,
	}
}

func (w *Worker) Start(ctx context.Context) {
	go w.run(ctx)
}

func (w *Worker) run(ctx context.Context) {
	w.runOnce(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *Worker) runOnce(ctx context.Context) {
	for _, destinationID := range w.service.DestinationIDs() {
		if err := ctx.Err(); err != nil {
			w.log.Warn(
				"destination healthcheck worker stopped",
				"error",
				err,
			)
			return
		}
		result, err := w.service.Trigger(
			ctx,
			destinationID,
		)
		if err != nil {
			w.log.Warn(
				"destination healthcheck failed",
				"error",
				err,
				"destination_id",
				destinationID,
			)
			continue
		}
		w.log.Info(
			"destination healthcheck completed",
			"destination_id",
			result.DestinationID,
			"status",
			result.Status,
		)
	}
}
