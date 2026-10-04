package trafficevents

import (
	"context"

	"github.com/devflex/traffoflex/packages/go-shared/eventstream"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type TrafficbackSink interface {
	WriteTrafficback(
		ctx context.Context,
		event models.TrafficbackEvent,
	) error
}

type KafkaTrafficbackSink struct {
	producer *eventstream.Producer
	topic    string
}

func NewKafkaTrafficbackSink(
	producer *eventstream.Producer,
	topic string,
) *KafkaTrafficbackSink {
	return &KafkaTrafficbackSink{
		producer: producer,
		topic:    topic,
	}
}

func (s *KafkaTrafficbackSink) WriteTrafficback(
	ctx context.Context,
	event models.TrafficbackEvent,
) error {
	return s.WriteBatch(
		ctx,
		[]models.TrafficbackEvent{event},
	)
}

func (s *KafkaTrafficbackSink) WriteBatch(
	ctx context.Context,
	events []models.TrafficbackEvent,
) error {
	items := make([]eventstream.JSONEvent, 0, len(events))
	for _, event := range events {
		items = append(items, eventstream.JSONEvent{
			Key: event.ClickID,
			Value: trafficbackRow{
				CreatedAt:           eventstream.ClickHouseDateTime(event.CreatedAt),
				OwnerID:             event.OwnerID,
				ClickID:             event.ClickID,
				CampaignID:          event.CampaignID,
				StreamID:            event.StreamID,
				DestinationID:       event.DestinationID,
				Reason:              string(event.Reason),
				Depth:               uint8(event.Depth),
				VisitedDestinations: event.VisitedDestinations,
			},
		})
	}
	return s.producer.WriteJSONBatch(ctx, s.topic, items)
}

type trafficbackRow struct {
	CreatedAt           string   `json:"created_at"`
	OwnerID             string   `json:"owner_id"`
	ClickID             string   `json:"click_id"`
	CampaignID          string   `json:"campaign_id"`
	StreamID            string   `json:"stream_id"`
	DestinationID       string   `json:"destination_id"`
	Reason              string   `json:"reason"`
	Depth               uint8    `json:"depth"`
	VisitedDestinations []string `json:"visited_destinations"`
}
