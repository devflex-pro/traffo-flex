package conversions

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type httpDoer interface {
	Do(req *http.Request) (
		*http.Response,
		error,
	)
}

type ClickHouseClickLookup struct {
	endpoint   string
	httpClient httpDoer
	timeout    time.Duration
}

func NewClickHouseClickLookup(
	endpoint string,
	timeout time.Duration,
) *ClickHouseClickLookup {
	return NewClickHouseClickLookupWithDoer(
		endpoint,
		timeout,
		nil,
	)
}

func NewClickHouseClickLookupWithDoer(
	endpoint string,
	timeout time.Duration,
	doer httpDoer,
) *ClickHouseClickLookup {
	if timeout <= 0 {
		timeout = time.Second
	}
	if doer == nil {
		doer = &http.Client{Timeout: timeout}
	}
	return &ClickHouseClickLookup{
		endpoint: strings.TrimRight(
			endpoint,
			"/",
		),
		httpClient: doer,
		timeout:    timeout,
	}
}

func (l *ClickHouseClickLookup) Find(
	ctx context.Context,
	clickID string,
) (
	ClickInfo,
	bool,
	error,
) {
	if strings.TrimSpace(l.endpoint) == "" {
		return ClickInfo{}, false, errors.New("clickhouse endpoint is required")
	}
	if strings.TrimSpace(clickID) == "" {
		return ClickInfo{}, false, nil
	}

	queryCtx, cancel := context.WithTimeout(
		ctx,
		l.timeout,
	)
	defer cancel()
	req, err := http.NewRequestWithContext(
		queryCtx,
		http.MethodPost,
		l.endpoint,
		strings.NewReader(buildClickLookupSQL(clickID)),
	)
	if err != nil {
		return ClickInfo{}, false, err
	}
	req.Header.Set(
		"Content-Type",
		"text/plain; charset=utf-8",
	)

	res, err := l.httpClient.Do(req)
	if err != nil {
		return ClickInfo{}, false, err
	}
	body, readErr := io.ReadAll(res.Body)
	closeErr := res.Body.Close()
	if readErr != nil {
		return ClickInfo{}, false, readErr
	}
	if closeErr != nil {
		return ClickInfo{}, false, closeErr
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ClickInfo{}, false, fmt.Errorf(
			"clickhouse returned status %d: %s",
			res.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	return decodeClickLookupRow(body)
}

func buildClickLookupSQL(clickID string) string {
	return fmt.Sprintf(
		`SELECT campaign_id, stream_id, destination_id, source_id
FROM click_events
WHERE click_id = '%s'
ORDER BY created_at DESC
LIMIT 1
FORMAT JSONEachRow`,
		escapeClickHouseString(clickID),
	)
}

func decodeClickLookupRow(body []byte) (
	ClickInfo,
	bool,
	error,
) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row clickLookupRow
		if err := json.Unmarshal(
			[]byte(line),
			&row,
		); err != nil {
			return ClickInfo{}, false, err
		}
		return ClickInfo(row), true, nil
	}
	if err := scanner.Err(); err != nil {
		return ClickInfo{}, false, err
	}
	return ClickInfo{}, false, nil
}

func escapeClickHouseString(value string) string {
	value = strings.ReplaceAll(
		value,
		`\`,
		`\\`,
	)
	return strings.ReplaceAll(
		value,
		"'",
		"''",
	)
}

type clickLookupRow struct {
	CampaignID    string `json:"campaign_id"`
	StreamID      string `json:"stream_id"`
	DestinationID string `json:"destination_id"`
	SourceID      string `json:"source_id"`
}
