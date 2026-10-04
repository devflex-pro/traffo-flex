package conversions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/eventstream"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type AttributionPending struct {
	Event    models.ConversionEvent
	Attempts int
}

type AttributionRepository interface {
	PendingAttribution(
		context.Context,
		int,
	) ([]AttributionPending, error)
	DeferAttribution(
		context.Context,
		string,
		int,
		time.Time,
	) error
	MarkAttributed(
		context.Context,
		string,
	) error
}

type ClickAttribution struct {
	OwnerID       string `json:"owner_id"`
	CampaignID    string `json:"campaign_id"`
	StreamID      string `json:"stream_id"`
	DestinationID string `json:"destination_id"`
	SourceID      string `json:"source_id"`
}

type AttributionLookup interface {
	Find(
		context.Context,
		string,
		string,
	) (ClickAttribution, bool, error)
}

type ClickHouseAttributionLookup struct {
	endpoint string
	client   *http.Client
}

func NewClickHouseAttributionLookup(endpoint string) *ClickHouseAttributionLookup {
	return &ClickHouseAttributionLookup{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 2 * time.Second},
	}
}

func (l *ClickHouseAttributionLookup) Find(
	ctx context.Context,
	ownerID string,
	clickID string,
) (ClickAttribution, bool, error) {
	// Click IDs originate from public postbacks. Escape the SQL string literal.
	quoted := strings.ReplaceAll(
		strings.ReplaceAll(
			clickID,
			"\\",
			"\\\\",
		),
		"'",
		"\\'",
	)
	query := "SELECT owner_id, campaign_id, stream_id, destination_id, source_id " +
		"FROM click_attribution_lookup WHERE click_id = '" + quoted + "'"
	if ownerID != "" {
		quotedOwner := strings.ReplaceAll(ownerID, "'", "\\'")
		query += " AND owner_id = '" + quotedOwner + "'"
	}
	query += " LIMIT 1 FORMAT JSONEachRow"
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		l.endpoint,
		strings.NewReader(query),
	)
	if err != nil {
		return ClickAttribution{}, false, err
	}
	response, err := l.client.Do(request)
	if err != nil {
		return ClickAttribution{}, false, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(
			response.Body,
			1024,
		))
		if readErr != nil {
			return ClickAttribution{}, false, errors.Join(
				fmt.Errorf(
					"clickhouse status %d",
					response.StatusCode,
				),
				readErr,
			)
		}
		return ClickAttribution{}, false, fmt.Errorf(
			"clickhouse status %d: %s",
			response.StatusCode,
			body,
		)
	}
	var row ClickAttribution
	if err := json.NewDecoder(response.Body).Decode(&row); err != nil {
		if errors.Is(
			err,
			io.EOF,
		) {
			return ClickAttribution{}, false, nil
		}
		return ClickAttribution{}, false, err
	}
	return row, true, nil
}

type AttributionSink interface {
	WriteAttributions(
		context.Context,
		[]AttributionEvent,
	) error
}

type AttributionEvent struct {
	Conversion models.ConversionEvent
	Click      ClickAttribution
}

type KafkaAttributionSink struct {
	producer *eventstream.Producer
	topic    string
}

func NewKafkaAttributionSink(
	producer *eventstream.Producer,
	topic string,
) *KafkaAttributionSink {
	return &KafkaAttributionSink{producer: producer, topic: topic}
}

func (s *KafkaAttributionSink) WriteAttributions(
	ctx context.Context,
	events []AttributionEvent,
) error {
	items := make([]eventstream.JSONEvent, 0, len(events))
	for _, event := range events {
		items = append(items, eventstream.JSONEvent{
			Key: event.Conversion.ConversionID,
			Value: map[string]any{
				"created_at":     eventstream.ClickHouseDateTime(event.Conversion.CreatedAt),
				"attributed_at":  eventstream.ClickHouseDateTime(time.Now().UTC()),
				"owner_id":       event.Conversion.OwnerID,
				"conversion_id":  event.Conversion.ConversionID,
				"click_id":       event.Conversion.ClickID,
				"campaign_id":    event.Click.CampaignID,
				"stream_id":      event.Click.StreamID,
				"destination_id": event.Click.DestinationID,
				"source_id":      event.Click.SourceID,
				"payout":         event.Conversion.Payout,
			},
		})
	}
	return s.producer.WriteJSONBatch(
		ctx,
		s.topic,
		items,
	)
}

type AttributionWorker struct {
	repo     AttributionRepository
	lookup   AttributionLookup
	sink     AttributionSink
	log      *slog.Logger
	interval time.Duration
}

func NewAttributionWorker(
	repo AttributionRepository,
	lookup AttributionLookup,
	sink AttributionSink,
	log *slog.Logger,
	interval time.Duration,
) *AttributionWorker {
	return &AttributionWorker{repo: repo, lookup: lookup, sink: sink, log: log, interval: interval}
}

func (w *AttributionWorker) Run(ctx context.Context) {
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

func (w *AttributionWorker) RunOnce(ctx context.Context) {
	pending, err := w.repo.PendingAttribution(
		ctx,
		100,
	)
	if err != nil {
		w.log.Warn(
			"pending attribution lookup failed",
			"error",
			err,
		)
		return
	}
	ready := make([]AttributionEvent, 0, len(pending))
	readyPending := make([]AttributionPending, 0, len(pending))
	for _, item := range pending {
		click, found, lookupErr := w.lookup.Find(
			ctx,
			item.Event.OwnerID,
			item.Event.ClickID,
		)
		if lookupErr == nil && found {
			ready = append(ready, AttributionEvent{Conversion: item.Event, Click: click})
			readyPending = append(readyPending, item)
			continue
		}
		if lookupErr != nil {
			w.log.Warn(
				"conversion attribution delayed",
				"conversion_id",
				item.Event.ConversionID,
				"error",
				lookupErr,
			)
		}
		if !w.deferPending(ctx, item) || lookupErr != nil {
			break
		}
	}
	if len(ready) == 0 {
		return
	}
	if err := w.sink.WriteAttributions(
		ctx,
		ready,
	); err != nil {
		w.log.Warn(
			"attribution batch delivery failed",
			"count",
			len(ready),
			"error",
			err,
		)
		for _, item := range readyPending {
			if !w.deferPending(ctx, item) {
				return
			}
		}
		return
	}
	for _, item := range readyPending {
		if err := w.repo.MarkAttributed(
			ctx,
			item.Event.ConversionID,
		); err != nil {
			w.log.Warn(
				"attribution checkpoint failed",
				"conversion_id",
				item.Event.ConversionID,
				"error",
				err,
			)
			return
		}
	}
}

func (w *AttributionWorker) deferPending(
	ctx context.Context,
	item AttributionPending,
) bool {
	attempts := item.Attempts + 1
	next := time.Now().UTC().Add(attributionBackoff(attempts))
	if err := w.repo.DeferAttribution(
		ctx,
		item.Event.ConversionID,
		attempts,
		next,
	); err != nil {
		w.log.Warn(
			"attribution retry checkpoint failed",
			"conversion_id",
			item.Event.ConversionID,
			"error",
			err,
		)
		return false
	}
	return true
}

func attributionBackoff(attempts int) time.Duration {
	delay := 5 * time.Second
	for attempt := 1; attempt < attempts && delay < 5*time.Minute; attempt++ {
		delay *= 2
	}
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}
