package clickaudit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/eventqueue"
)

const (
	dailyInterval        = 24 * time.Hour
	retryInterval        = 5 * time.Minute
	recoveryGrace        = time.Minute
	recoveryConfirmation = 5 * time.Minute
	auditTimeout         = 30 * time.Second
)

type WALSource interface {
	RecoveryWindow() (time.Time, time.Time, int64)
	Stats() eventqueue.Stats
}

type Result struct {
	WindowStart         time.Time `json:"window_start"`
	WindowEnd           time.Time `json:"window_end"`
	CheckedAt           time.Time `json:"checked_at"`
	DuplicateEvents     uint64    `json:"duplicate_events"`
	ExcessCost          float64   `json:"excess_cost"`
	ConflictingClickIDs uint64    `json:"conflicting_click_ids"`
	Error               string    `json:"error,omitempty"`
}

type Stats struct {
	Daily           Result `json:"daily"`
	Recovery        Result `json:"recovery"`
	RecoveredClicks int64  `json:"recovered_clicks"`
	RecoveryPending bool   `json:"recovery_pending"`
	RunTotal        uint64 `json:"run_total"`
	FailureTotal    uint64 `json:"failure_total"`
}

type doer interface {
	Do(*http.Request) (*http.Response, error)
}

type Auditor struct {
	endpoint     string
	client       doer
	log          *slog.Logger
	retry        time.Duration
	grace        time.Duration
	confirmation time.Duration
	poll         time.Duration
	runMu        sync.Mutex
	mu           sync.Mutex
	stats        Stats
}

func New(endpoint string, log *slog.Logger) *Auditor {
	return NewWithDoer(
		endpoint,
		log,
		&http.Client{Timeout: auditTimeout},
	)
}

func NewWithDoer(
	endpoint string,
	log *slog.Logger,
	client doer,
) *Auditor {
	if log == nil {
		log = slog.Default()
	}
	if client == nil {
		client = &http.Client{Timeout: auditTimeout}
	}
	return &Auditor{
		endpoint:     endpoint,
		client:       client,
		log:          log,
		retry:        retryInterval,
		grace:        recoveryGrace,
		confirmation: recoveryConfirmation,
		poll:         time.Second,
	}
}

func (a *Auditor) Stats() Stats {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stats
}

func (a *Auditor) Start(ctx context.Context, wal WALSource) {
	go a.daily(ctx)
	if wal == nil {
		return
	}
	from, to, recovered := wal.RecoveryWindow()
	if recovered == 0 {
		return
	}
	a.mu.Lock()
	a.stats.RecoveredClicks = recovered
	a.stats.RecoveryPending = true
	a.mu.Unlock()
	if from.IsZero() || to.IsZero() {
		a.mu.Lock()
		a.stats.RecoveryPending = false
		a.stats.Recovery.Error = "recovered click timestamps are missing"
		a.mu.Unlock()
		a.log.Error("click WAL recovery has no event timestamps", "recovered_clicks", recovered)
		return
	}
	go a.recovery(ctx, wal, dayStart(from), dayStart(to).Add(24*time.Hour))
}

func (a *Auditor) RunOnce(
	ctx context.Context,
	kind string,
	from time.Time,
	to time.Time,
) error {
	if kind != "daily" && kind != "recovery" {
		return errors.New("invalid click audit kind")
	}
	if !from.Before(to) {
		return errors.New("invalid click audit window")
	}
	a.runMu.Lock()
	defer a.runMu.Unlock()
	result := Result{
		WindowStart: from.UTC(),
		WindowEnd:   to.UTC(),
		CheckedAt:   time.Now().UTC(),
	}
	checkCtx, cancel := context.WithTimeout(ctx, auditTimeout)
	defer cancel()
	duplicateEvents, excessCost, conflicts, err := a.query(
		checkCtx,
		from,
		to,
	)
	if err != nil {
		result.Error = err.Error()
	} else {
		result.DuplicateEvents = duplicateEvents
		result.ExcessCost = excessCost
		result.ConflictingClickIDs = conflicts
	}
	a.mu.Lock()
	a.stats.RunTotal++
	if err != nil {
		a.stats.FailureTotal++
	}
	if kind == "daily" {
		a.stats.Daily = result
	} else {
		a.stats.Recovery = result
	}
	a.mu.Unlock()
	if err != nil {
		a.log.Error("click duplicate audit failed", "kind", kind, "error", err)
		return err
	}
	if duplicateEvents > 0 || conflicts > 0 {
		a.log.Warn(
			"duplicate click events found",
			"kind", kind,
			"from", result.WindowStart,
			"to", result.WindowEnd,
			"duplicate_events", duplicateEvents,
			"excess_cost", excessCost,
			"conflicting_click_ids", conflicts,
		)
	}
	return nil
}

func (a *Auditor) daily(ctx context.Context) {
	for {
		now := time.Now().UTC()
		from := dayStart(now).Add(-24 * time.Hour)
		to := dayStart(now).Add(24 * time.Hour)
		err := a.RunOnce(ctx, "daily", from, to)
		delay := dailyInterval
		if err != nil {
			delay = a.retry
		}
		if !wait(ctx, delay) {
			return
		}
	}
}

func (a *Auditor) recovery(
	ctx context.Context,
	wal WALSource,
	from time.Time,
	to time.Time,
) {
	ticker := time.NewTicker(a.poll)
	defer ticker.Stop()
	for wal.Stats().RecoveryPending {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
	if !wait(ctx, a.grace) {
		return
	}
	for {
		if err := a.RunOnce(ctx, "recovery", from, to); err == nil {
			break
		}
		if !wait(ctx, a.retry) {
			return
		}
	}
	if !wait(ctx, a.confirmation) {
		return
	}
	for {
		if err := a.RunOnce(ctx, "recovery", from, to); err == nil {
			a.mu.Lock()
			a.stats.RecoveryPending = false
			a.mu.Unlock()
			return
		}
		if !wait(ctx, a.retry) {
			return
		}
	}
}

func (a *Auditor) query(
	ctx context.Context,
	from time.Time,
	to time.Time,
) (uint64, float64, uint64, error) {
	sql := fmt.Sprintf(
		`SELECT toString(sum(duplicate_rows)) AS duplicate_events,
toString(sum(excess_cost)) AS excess_cost,
toString(countIf(field_conflict)) AS conflicting_click_ids
FROM (
  SELECT click_id, count() - 1 AS duplicate_rows,
    sum(cost) - min(cost) AS excess_cost,
    (min(cost) != max(cost) OR
     uniqExact((created_at, campaign_id, stream_id, destination_id, source_id)) > 1) AS field_conflict
  FROM click_events
  WHERE created_at >= toDateTime('%s') AND created_at < toDateTime('%s')
  GROUP BY click_id
  HAVING count() > 1
)
FORMAT JSONEachRow`,
		from.UTC().Format(time.DateTime),
		to.UTC().Format(time.DateTime),
	)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		a.endpoint,
		strings.NewReader(sql),
	)
	if err != nil {
		return 0, 0, 0, err
	}
	req.Header.Set("Content-Type", "text/plain")
	res, err := a.client.Do(req)
	if err != nil {
		return 0, 0, 0, err
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	closeErr := res.Body.Close()
	if err != nil || closeErr != nil {
		return 0, 0, 0, errors.Join(err, closeErr)
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return 0, 0, 0, fmt.Errorf("clickhouse audit status %d: %s", res.StatusCode, strings.TrimSpace(string(data)))
	}
	var row struct {
		DuplicateEvents     string `json:"duplicate_events"`
		ExcessCost          string `json:"excess_cost"`
		ConflictingClickIDs string `json:"conflicting_click_ids"`
	}
	if err := json.Unmarshal(data, &row); err != nil {
		return 0, 0, 0, err
	}
	duplicates, err := strconv.ParseUint(row.DuplicateEvents, 10, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	cost, err := strconv.ParseFloat(row.ExcessCost, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	conflicts, err := strconv.ParseUint(row.ConflictingClickIDs, 10, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	return duplicates, cost, conflicts, nil
}

func dayStart(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(
		utc.Year(),
		utc.Month(),
		utc.Day(),
		0,
		0,
		0,
		0,
		time.UTC,
	)
}

func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
