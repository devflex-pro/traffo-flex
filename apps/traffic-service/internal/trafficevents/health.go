package trafficevents

import (
	"context"
	"log/slog"

	"github.com/devflex/traffoflex/packages/go-shared/eventstream"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type DestinationHealthSink interface {
	WriteDestinationHealth(
		ctx context.Context,
		event models.DestinationHealthEvent,
	) error
}

type BestEffortDestinationHealthSink struct {
	log  *slog.Logger
	next DestinationHealthSink
}

func NewBestEffortDestinationHealthSink(
	log *slog.Logger,
	next DestinationHealthSink,
) *BestEffortDestinationHealthSink {
	return &BestEffortDestinationHealthSink{
		log:  log,
		next: next,
	}
}

func (s *BestEffortDestinationHealthSink) WriteDestinationHealth(
	ctx context.Context,
	event models.DestinationHealthEvent,
) error {
	if s.next == nil {
		return nil
	}
	if err := s.next.WriteDestinationHealth(
		ctx,
		event,
	); err != nil {
		if s.log != nil {
			s.log.Warn(
				"destination health event write failed",
				"error",
				err,
				"destination_id",
				event.DestinationID,
			)
		}
	}
	return nil
}

type KafkaDestinationHealthSink struct {
	producer *eventstream.Producer
	topic    string
}

func NewKafkaDestinationHealthSink(
	producer *eventstream.Producer,
	topic string,
) *KafkaDestinationHealthSink {
	return &KafkaDestinationHealthSink{
		producer: producer,
		topic:    topic,
	}
}

func (s *KafkaDestinationHealthSink) WriteDestinationHealth(
	ctx context.Context,
	event models.DestinationHealthEvent,
) error {
	return s.producer.WriteJSON(
		ctx,
		s.topic,
		event.DestinationID,
		destinationHealthRow{
			CreatedAt:     eventstream.ClickHouseDateTime(event.CreatedAt),
			DestinationID: event.DestinationID,
			Previous:      string(event.Previous),
			Current:       string(event.Current),
			Error:         event.Error,
		},
	)
}

type destinationHealthRow struct {
	CreatedAt     string `json:"created_at"`
	DestinationID string `json:"destination_id"`
	Previous      string `json:"previous"`
	Current       string `json:"current"`
	Error         string `json:"error"`
}
