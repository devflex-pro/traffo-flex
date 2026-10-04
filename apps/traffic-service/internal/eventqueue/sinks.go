package eventqueue

import (
	"context"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/clicklog"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/trafficevents"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type ClickLogger struct {
	queue *Queue[models.ClickEvent]
}

func NewClickLogger(queue *Queue[models.ClickEvent]) *ClickLogger {
	return &ClickLogger{queue: queue}
}

func (l *ClickLogger) Log(
	ctx context.Context,
	event models.ClickEvent,
) error {
	return l.queue.Submit(ctx, event)
}

type TrafficbackSink struct {
	queue *Queue[models.TrafficbackEvent]
}

func NewTrafficbackSink(queue *Queue[models.TrafficbackEvent]) *TrafficbackSink {
	return &TrafficbackSink{queue: queue}
}

func (s *TrafficbackSink) WriteTrafficback(
	ctx context.Context,
	event models.TrafficbackEvent,
) error {
	return s.queue.Submit(ctx, event)
}

var (
	_ clicklog.Logger               = (*ClickLogger)(nil)
	_ trafficevents.TrafficbackSink = (*TrafficbackSink)(nil)
)
