package antirepeat

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

	"github.com/devflex/traffoflex/apps/traffic-service/internal/availability"
	"github.com/devflex/traffoflex/apps/traffic-service/internal/cache"
)

type Backfill struct {
	endpoint string
	client   *http.Client
}

func NewBackfill(endpoint string) *Backfill {
	return &Backfill{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func (b *Backfill) Seed(
	ctx context.Context,
	item PendingStream,
	manager *Manager,
) (resultErr error) {
	expression, err := availability.UserKeyExpression(item.UserKey)
	if err != nil {
		return err
	}
	if item.WindowHours <= 0 {
		return errors.New("anti-repeat history window must be positive")
	}
	query := fmt.Sprintf(
		"SELECT toUnixTimestamp(created_at) AS event_time, destination_id, %s AS user_value FROM click_events WHERE campaign_id = '%s' AND stream_id = '%s' AND created_at >= now() - INTERVAL %d HOUR FORMAT JSONEachRow",
		expression,
		quote(item.Key.CampaignID),
		quote(item.Key.StreamID),
		item.WindowHours,
	)
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		b.endpoint,
		strings.NewReader(query),
	)
	if err != nil {
		return err
	}
	request.Header.Set(
		"Content-Type",
		"text/plain; charset=utf-8",
	)
	response, err := b.client.Do(request)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, response.Body.Close()) }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(
			response.Body,
			4096,
		))
		return errors.Join(
			fmt.Errorf("clickhouse history returned %d: %s", response.StatusCode, body),
			readErr,
		)
	}
	decoder := json.NewDecoder(response.Body)
	for {
		var row struct {
			EventTime     int64  `json:"event_time"`
			DestinationID string `json:"destination_id"`
			UserValue     string `json:"user_value"`
		}
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if err := manager.MarkHistorical(
			item.Key,
			item.UserKey,
			row.UserValue,
			row.DestinationID,
			time.Unix(row.EventTime, 0).UTC(),
		); err != nil {
			return err
		}
	}
}

func quote(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func (m *Manager) Start(
	ctx context.Context,
	store *cache.Store,
	source *Backfill,
	log *slog.Logger,
) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		period := time.NewTicker(m.config.SnapshotInterval)
		defer period.Stop()
		var reloadedAt time.Time
		refresh := func() {
			if latest := store.ReloadedAt(); !latest.Equal(reloadedAt) {
				m.Configure(store.SnapshotCampaigns())
				reloadedAt = latest
			}
			for _, item := range m.Pending() {
				seedCtx, cancel := context.WithTimeout(
					ctx,
					5*time.Minute,
				)
				err := source.Seed(
					seedCtx,
					item,
					m,
				)
				cancel()
				if err != nil {
					m.SetSeedError(err)
					log.Warn(
						"anti-repeat history backfill failed",
						"stream_id",
						item.Key.StreamID,
						"error",
						err,
					)
					continue
				}
				m.SetReady(
					item.Key,
					item.UserKey,
					item.WindowHours,
				)
				m.SetSeedError(nil)
			}
			if err := m.Save(); err != nil {
				log.Error(
					"anti-repeat snapshot save failed",
					"error",
					err,
				)
			}
		}
		refresh()
		for {
			select {
			case <-ctx.Done():
				return
			case <-period.C:
				refresh()
			}
		}
	}()
	return done
}
