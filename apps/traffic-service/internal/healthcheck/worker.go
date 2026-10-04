package healthcheck

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Worker struct {
	log         *slog.Logger
	service     *Service
	interval    time.Duration
	parallelism int
}

func NewWorker(
	log *slog.Logger,
	service *Service,
	interval time.Duration,
	parallelism int,
) *Worker {
	if interval <= 0 {
		interval = time.Minute
	}
	if parallelism <= 0 {
		parallelism = 4
	}
	return &Worker{
		log:         log,
		service:     service,
		interval:    interval,
		parallelism: parallelism,
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
	sem := make(chan struct{}, w.parallelism)
	var pending sync.WaitGroup
	defer pending.Wait()
	for _, destinationID := range w.service.DestinationIDs() {
		select {
		case <-ctx.Done():
			return
		case sem <- struct{}{}:
		}
		pending.Add(1)
		go func(id string) {
			defer pending.Done()
			defer func() { <-sem }()
			result, err := w.service.Trigger(ctx, id)
			if err != nil {
				w.log.Warn("destination healthcheck failed", "destination_id", id, "error", err)
				return
			}
			w.log.Info("destination healthcheck completed", "destination_id", result.DestinationID, "status", result.Status, "probe_error", result.Error)
		}(destinationID)
	}
}
