package availability

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

const defaultCapCacheTTL = 10 * time.Second

type ClickHouseCapChecker struct {
	endpoint string
	client   httpDoer
	ttl      time.Duration
	mu       sync.Mutex
	cache    map[string]capCacheEntry
	history  map[string]historyCacheEntry
	roi      map[string]roiCacheEntry
}

type httpDoer interface {
	Do(req *http.Request) (
		*http.Response,
		error,
	)
}

type capCacheEntry struct {
	exceeded  bool
	expiresAt time.Time
}

type historyCacheEntry struct {
	used      map[string]struct{}
	expiresAt time.Time
}

type roiCacheEntry struct {
	ranked    []string
	expiresAt time.Time
}

func NewClickHouseCapChecker(endpoint string) *ClickHouseCapChecker {
	return NewClickHouseCapCheckerWithDoer(
		endpoint,
		&http.Client{Timeout: 750 * time.Millisecond},
		defaultCapCacheTTL,
	)
}

func NewClickHouseCapCheckerWithDoer(
	endpoint string,
	doer httpDoer,
	ttl time.Duration,
) *ClickHouseCapChecker {
	if doer == nil {
		doer = &http.Client{Timeout: 750 * time.Millisecond}
	}
	if ttl <= 0 {
		ttl = defaultCapCacheTTL
	}
	return &ClickHouseCapChecker{
		endpoint: strings.TrimRight(
			endpoint,
			"/",
		),
		client:  doer,
		ttl:     ttl,
		cache:   make(map[string]capCacheEntry),
		history: make(map[string]historyCacheEntry),
		roi:     make(map[string]roiCacheEntry),
	}
}

func (c *ClickHouseCapChecker) Exceeded(
	ctx context.Context,
	destination models.Destination,
	now time.Time,
) (
	bool,
	error,
) {
	if strings.TrimSpace(c.endpoint) == "" {
		return false, errors.New("clickhouse endpoint is required")
	}
	for _, rule := range destination.Caps.Rules {
		if err := models.ValidateDestinationCapRule(rule); err != nil {
			return false, err
		}
		exceeded, ok := c.cached(
			destination.ID,
			rule,
			now,
		)
		if ok {
			if exceeded {
				return true, nil
			}
			continue
		}
		value, err := c.queryRule(
			ctx,
			destination.ID,
			rule,
		)
		if err != nil {
			return false, err
		}
		exceeded = value >= rule.Limit
		c.store(
			destination.ID,
			rule,
			exceeded,
			now,
		)
		if exceeded {
			return true, nil
		}
	}
	return false, nil
}

func (c *ClickHouseCapChecker) UsedDestinations(
	ctx context.Context,
	userKey string,
	userValue string,
	windowHours int,
	now time.Time,
) (
	map[string]struct{},
	error,
) {
	if strings.TrimSpace(c.endpoint) == "" {
		return nil, errors.New("clickhouse endpoint is required")
	}
	expression, err := userKeyExpression(userKey)
	if err != nil {
		return nil, err
	}
	key := "history|" + userKey + "|" + userValue + "|" + strconv.Itoa(windowHours)
	if used, ok := c.cachedHistory(
		key,
		now,
	); ok {
		return used, nil
	}
	body, err := c.querySQL(
		ctx,
		fmt.Sprintf(
			"SELECT DISTINCT destination_id FROM click_events WHERE %s = %s AND created_at >= now() - INTERVAL %d HOUR FORMAT JSONEachRow",
			expression,
			quoteString(userValue),
			windowHours,
		),
	)
	if err != nil {
		return nil, err
	}
	used, err := decodeUsedDestinations(body)
	if err != nil {
		return nil, err
	}
	c.storeHistory(
		key,
		used,
		now,
	)
	return used, nil
}

func (c *ClickHouseCapChecker) RankByROI(
	ctx context.Context,
	destinationIDs []string,
	windowHours int,
	minClicks int,
	now time.Time,
) (
	[]string,
	error,
) {
	if strings.TrimSpace(c.endpoint) == "" {
		return nil, errors.New("clickhouse endpoint is required")
	}
	if len(destinationIDs) == 0 {
		return nil, nil
	}
	key := "roi|" + strings.Join(
		destinationIDs,
		",",
	) + "|" + strconv.Itoa(windowHours) + "|" + strconv.Itoa(minClicks)
	if ranked, ok := c.cachedROI(
		key,
		now,
	); ok {
		return ranked, nil
	}
	body, err := c.querySQL(
		ctx,
		buildROISQL(
			destinationIDs,
			windowHours,
		),
	)
	if err != nil {
		return nil, err
	}
	ranked, err := decodeROIRanking(
		body,
		minClicks,
	)
	if err != nil {
		return nil, err
	}
	c.storeROI(
		key,
		ranked,
		now,
	)
	return ranked, nil
}

func (c *ClickHouseCapChecker) cached(
	destinationID string,
	rule models.DestinationCapRule,
	now time.Time,
) (
	bool,
	bool,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.cache[cacheKey(
		destinationID,
		rule,
	)]
	if !ok || now.After(entry.expiresAt) {
		return false, false
	}
	return entry.exceeded, true
}

func (c *ClickHouseCapChecker) store(
	destinationID string,
	rule models.DestinationCapRule,
	exceeded bool,
	now time.Time,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[cacheKey(
		destinationID,
		rule,
	)] = capCacheEntry{
		exceeded:  exceeded,
		expiresAt: now.Add(c.ttl),
	}
}

func cacheKey(
	destinationID string,
	rule models.DestinationCapRule,
) string {
	return destinationID + "|" +
		string(rule.Metric) + "|" +
		strconv.Itoa(rule.WindowHours) + "|" +
		strconv.FormatFloat(
			rule.Limit,
			'f',
			-1,
			64,
		)
}

func (c *ClickHouseCapChecker) queryRule(
	ctx context.Context,
	destinationID string,
	rule models.DestinationCapRule,
) (
	float64,
	error,
) {
	body, err := c.querySQL(
		ctx,
		buildCapSQL(
			destinationID,
			rule,
		),
	)
	if err != nil {
		return 0, err
	}
	return decodeCapValue(body)
}

func (c *ClickHouseCapChecker) querySQL(
	ctx context.Context,
	sql string,
) (
	[]byte,
	error,
) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.endpoint,
		strings.NewReader(sql),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set(
		"Content-Type",
		"text/plain; charset=utf-8",
	)
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(res.Body)
	closeErr := res.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf(
			"clickhouse returned status %d: %s",
			res.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}
	return body, nil
}

func buildCapSQL(
	destinationID string,
	rule models.DestinationCapRule,
) string {
	table := "click_events"
	expression := "count()"
	switch rule.Metric {
	case models.CapMetricCost:
		expression = "sum(cost)"
	case models.CapMetricConversions:
		table = "conversion_events"
		expression = "count()"
	case models.CapMetricRevenue:
		table = "conversion_events"
		expression = "sum(payout)"
	}
	return fmt.Sprintf(
		"SELECT %s AS value FROM %s WHERE destination_id = %s AND created_at >= now() - INTERVAL %d HOUR FORMAT JSONEachRow",
		expression,
		table,
		quoteString(destinationID),
		rule.WindowHours,
	)
}

func buildROISQL(
	destinationIDs []string,
	windowHours int,
) string {
	quotedIDs := make(
		[]string,
		0,
		len(destinationIDs),
	)
	for _, id := range destinationIDs {
		quotedIDs = append(
			quotedIDs,
			quoteString(id),
		)
	}
	inList := strings.Join(
		quotedIDs,
		",",
	)
	return fmt.Sprintf(
		`SELECT destination_id, sum(clicks) AS clicks, sum(conversions) AS conversions, sum(revenue) AS revenue, sum(cost) AS cost
FROM (
  SELECT destination_id, count() AS clicks, 0 AS conversions, 0.0 AS revenue, sum(cost) AS cost
  FROM click_events
  WHERE destination_id IN (%s) AND created_at >= now() - INTERVAL %d HOUR
  GROUP BY destination_id
  UNION ALL
  SELECT destination_id, 0 AS clicks, count() AS conversions, sum(payout) AS revenue, 0.0 AS cost
  FROM conversion_events
  WHERE destination_id IN (%s) AND created_at >= now() - INTERVAL %d HOUR
  GROUP BY destination_id
)
GROUP BY destination_id
FORMAT JSONEachRow`,
		inList,
		windowHours,
		inList,
		windowHours,
	)
}

func userKeyExpression(userKey string) (
	string,
	error,
) {
	switch userKey {
	case "source_click_id", "sub1", "sub2", "sub3", "sub4", "sub5", "sub6", "sub7", "sub8", "sub9", "sub10",
		"utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term", "user_agent":
		return userKey, nil
	}
	if strings.HasPrefix(
		userKey,
		"query.",
	) {
		field := strings.TrimPrefix(
			userKey,
			"query.",
		)
		if field == "" || strings.ContainsAny(
			field,
			"'\\",
		) {
			return "", errors.New("user key is invalid")
		}
		return "JSONExtractString(query, " + quoteString(field) + ")", nil
	}
	return "", errors.New("user key is unsupported by clickhouse history")
}

func quoteString(value string) string {
	return "'" + strings.ReplaceAll(
		value,
		"'",
		"''",
	) + "'"
}

func decodeCapValue(body []byte) (
	float64,
	error,
) {
	var row struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(
		body,
		&row,
	); err != nil {
		return 0, err
	}
	if len(row.Value) == 0 {
		return 0, nil
	}
	var number float64
	if err := json.Unmarshal(
		row.Value,
		&number,
	); err == nil {
		if math.IsNaN(number) || math.IsInf(
			number,
			0,
		) {
			return 0, nil
		}
		return number, nil
	}
	var text string
	if err := json.Unmarshal(
		row.Value,
		&text,
	); err != nil {
		return 0, err
	}
	if strings.TrimSpace(text) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(
		text,
		64,
	)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func decodeUsedDestinations(body []byte) (
	map[string]struct{},
	error,
) {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	used := make(map[string]struct{})
	for {
		var row struct {
			DestinationID string `json:"destination_id"`
		}
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(
				err,
				io.EOF,
			) {
				break
			}
			return nil, err
		}
		if row.DestinationID != "" {
			used[row.DestinationID] = struct{}{}
		}
	}
	return used, nil
}

func decodeROIRanking(
	body []byte,
	minClicks int,
) (
	[]string,
	error,
) {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	rows := make([]roiRow, 0)
	for {
		var row roiRow
		if err := decoder.Decode(&row); err != nil {
			if errors.Is(
				err,
				io.EOF,
			) {
				break
			}
			return nil, err
		}
		if row.DestinationID == "" || row.Clicks < minClicks || row.Cost <= 0 {
			continue
		}
		rows = append(
			rows,
			row,
		)
	}
	sort.SliceStable(
		rows,
		func(
			i,
			j int,
		) bool {
			return rows[i].roi() > rows[j].roi()
		},
	)
	ranked := make(
		[]string,
		0,
		len(rows),
	)
	for _, row := range rows {
		ranked = append(
			ranked,
			row.DestinationID,
		)
	}
	return ranked, nil
}

type roiRow struct {
	DestinationID string  `json:"destination_id"`
	Clicks        int     `json:"clicks"`
	Conversions   int     `json:"conversions"`
	Revenue       float64 `json:"revenue"`
	Cost          float64 `json:"cost"`
}

func (r roiRow) roi() float64 {
	return (r.Revenue - r.Cost) / r.Cost * 100
}

func (c *ClickHouseCapChecker) cachedHistory(
	key string,
	now time.Time,
) (
	map[string]struct{},
	bool,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.history[key]
	if !ok || now.After(entry.expiresAt) {
		return nil, false
	}
	used := make(
		map[string]struct{},
		len(entry.used),
	)
	for id := range entry.used {
		used[id] = struct{}{}
	}
	return used, true
}

func (c *ClickHouseCapChecker) storeHistory(
	key string,
	used map[string]struct{},
	now time.Time,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history[key] = historyCacheEntry{
		used:      used,
		expiresAt: now.Add(c.ttl),
	}
}

func (c *ClickHouseCapChecker) cachedROI(
	key string,
	now time.Time,
) (
	[]string,
	bool,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.roi[key]
	if !ok || now.After(entry.expiresAt) {
		return nil, false
	}
	ranked := make(
		[]string,
		len(entry.ranked),
	)
	copy(
		ranked,
		entry.ranked,
	)
	return ranked, true
}

func (c *ClickHouseCapChecker) storeROI(
	key string,
	ranked []string,
	now time.Time,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.roi[key] = roiCacheEntry{
		ranked:    ranked,
		expiresAt: now.Add(c.ttl),
	}
}
