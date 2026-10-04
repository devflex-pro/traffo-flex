package conversions

import (
	"context"
	"log/slog"
	"time"
)

type Worker struct {
	repo     Repository
	sink     EventSink
	log      *slog.Logger
	interval time.Duration
}

func NewWorker(
	repo Repository,
	sink EventSink,
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
			"pending conversions lookup failed",
			"error",
			err,
		)
		return
	}
	for _, event := range events {
		if err := w.sink.Write(
			ctx,
			event,
		); err != nil {
			w.log.Warn(
				"conversion delivery failed",
				"conversion_id",
				event.ConversionID,
				"error",
				err,
			)
			return
		}
		if err := w.repo.MarkDelivered(
			ctx,
			event.ConversionID,
		); err != nil {
			w.log.Warn(
				"conversion delivery checkpoint failed",
				"conversion_id",
				event.ConversionID,
				"error",
				err,
			)
			return
		}
	}
}
