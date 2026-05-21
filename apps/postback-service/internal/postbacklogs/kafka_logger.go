package postbacklogs

import (
	"context"
	"encoding/json"

	"github.com/devflex/traffoflex/packages/go-shared/eventstream"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type KafkaLogger struct {
	producer *eventstream.Producer
	topic    string
}

func NewKafkaLogger(
	producer *eventstream.Producer,
	topic string,
) *KafkaLogger {
	return &KafkaLogger{producer: producer, topic: topic}
}

func (l *KafkaLogger) Log(
	ctx context.Context,
	event models.PostbackLogEvent,
) error {
	rawPayload, err := json.Marshal(event.RawPayload)
	if err != nil {
		return err
	}
	return l.producer.WriteJSON(
		ctx,
		l.topic,
		event.PostbackID,
		postbackLogRow{
			CreatedAt:     eventstream.ClickHouseDateTime(event.CreatedAt),
			PostbackID:    event.PostbackID,
			NetworkID:     event.NetworkID,
			ClickID:       event.ClickID,
			TransactionID: event.TransactionID,
			Status:        event.Status,
			Error:         event.Error,
			RawPayload:    string(rawPayload),
		},
	)
}

type postbackLogRow struct {
	CreatedAt     string `json:"created_at"`
	PostbackID    string `json:"postback_id"`
	NetworkID     string `json:"network_id"`
	ClickID       string `json:"click_id"`
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	Error         string `json:"error"`
	RawPayload    string `json:"raw_payload"`
}
