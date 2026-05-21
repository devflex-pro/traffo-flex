package conversions

import (
	"context"
	"encoding/json"

	"github.com/devflex/traffoflex/packages/go-shared/eventstream"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type KafkaSink struct {
	producer *eventstream.Producer
	topic    string
}

func NewKafkaSink(
	producer *eventstream.Producer,
	topic string,
) *KafkaSink {
	return &KafkaSink{producer: producer, topic: topic}
}

func (s *KafkaSink) Write(
	ctx context.Context,
	event models.ConversionEvent,
) error {
	rawPayload, err := json.Marshal(event.RawPayload)
	if err != nil {
		return err
	}
	return s.producer.WriteJSON(
		ctx,
		s.topic,
		event.ConversionID,
		conversionRow{
			CreatedAt:     eventstream.ClickHouseDateTime(event.CreatedAt),
			UpdatedAt:     eventstream.ClickHouseDateTime(event.UpdatedAt),
			ConversionID:  event.ConversionID,
			ClickID:       event.ClickID,
			TransactionID: event.TransactionID,
			CampaignID:    event.CampaignID,
			StreamID:      event.StreamID,
			DestinationID: event.DestinationID,
			SourceID:      event.SourceID,
			OfferID:       event.OfferID,
			EventType:     event.EventType,
			Status:        event.Status,
			Payout:        event.Payout,
			Currency:      event.Currency,
			NetworkID:     event.NetworkID,
			RawPayload:    string(rawPayload),
		},
	)
}

type conversionRow struct {
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
	ConversionID  string  `json:"conversion_id"`
	ClickID       string  `json:"click_id"`
	TransactionID string  `json:"transaction_id"`
	CampaignID    string  `json:"campaign_id"`
	StreamID      string  `json:"stream_id"`
	DestinationID string  `json:"destination_id"`
	SourceID      string  `json:"source_id"`
	OfferID       string  `json:"offer_id"`
	EventType     string  `json:"event_type"`
	Status        string  `json:"status"`
	Payout        float64 `json:"payout"`
	Currency      string  `json:"currency"`
	NetworkID     string  `json:"network_id"`
	RawPayload    string  `json:"raw_payload"`
}
