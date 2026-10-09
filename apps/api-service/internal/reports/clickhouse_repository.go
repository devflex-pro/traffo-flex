package reports

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/devflex/traffoflex/apps/api-service/internal/scope"
)

const (
	clickEventsTable       = "click_events"
	clickStatsDayTable     = "click_stats_1d"
	clickStatsHourTable    = "click_stats_1h"
	conversionEventsTable  = "conversion_events"
	conversionReportSource = `(SELECT c.created_at AS created_at, c.payout AS payout, c.owner_id AS owner_id, c.click_id AS click_id,
  k.campaign_id AS campaign_id, k.stream_id AS stream_id,
  k.destination_id AS destination_id, k.source_id AS source_id
FROM (SELECT owner_id, conversion_id, any(created_at) AS created_at,
  any(payout) AS payout, any(click_id) AS click_id FROM conversion_events GROUP BY owner_id, conversion_id) AS c
LEFT ANY JOIN (SELECT owner_id, conversion_id, any(campaign_id) AS campaign_id,
  any(stream_id) AS stream_id, any(destination_id) AS destination_id,
  any(source_id) AS source_id FROM attributed_conversion_events GROUP BY owner_id, conversion_id) AS k
ON c.owner_id = k.owner_id AND c.conversion_id = k.conversion_id)`
	trafficbackEventsTable       = "trafficback_events"
	destinationHealthEventsTable = "destination_health_events"
	kafkaIngestionErrorsTable    = "kafka_ingestion_errors"
	defaultReportLimit           = 100
)

type ClickHouseRepository struct {
	endpoint   string
	httpClient httpDoer
}

type httpDoer interface {
	Do(req *http.Request) (
		*http.Response,
		error,
	)
}

func NewClickHouseRepository(endpoint string) *ClickHouseRepository {
	return NewClickHouseRepositoryWithDoer(
		endpoint,
		&http.Client{Timeout: 10 * time.Second},
	)
}

func NewClickHouseRepositoryWithDoer(
	endpoint string,
	doer httpDoer,
) *ClickHouseRepository {
	if doer == nil {
		doer = &http.Client{Timeout: 10 * time.Second}
	}
	return &ClickHouseRepository{
		endpoint: strings.TrimRight(
			endpoint,
			"/",
		),
		httpClient: doer,
	}
}

func (r *ClickHouseRepository) Overview(
	ctx context.Context,
	query Query,
) (
	Metrics,
	error,
) {
	query.OwnerID = scope.OwnerID(ctx)
	rows, err := r.queryMetrics(
		ctx,
		buildOverviewSQL(query),
	)
	if err != nil {
		return Metrics{}, err
	}
	if len(rows) == 0 {
		return Metrics{}, nil
	}
	return rows[0].Metrics(), nil
}

func (r *ClickHouseRepository) Grouped(
	ctx context.Context,
	groupBy GroupBy,
	query Query,
) (
	GroupedReport,
	error,
) {
	query.OwnerID = scope.OwnerID(ctx)
	rows, err := r.queryMetrics(
		ctx,
		buildGroupedSQL(
			groupBy,
			query,
		),
	)
	if err != nil {
		return GroupedReport{}, err
	}

	reportRows := make(
		[]ReportRow,
		0,
		len(rows),
	)
	var summary Metrics
	for _, row := range rows {
		metrics := row.Metrics()
		reportRows = append(
			reportRows,
			ReportRow{
				ID:      row.ID,
				Name:    row.Name,
				Metrics: metrics,
			},
		)
		summary.Clicks += metrics.Clicks
		summary.Conversions += metrics.Conversions
		summary.Revenue += metrics.Revenue
		summary.Cost += metrics.Cost
		summary.Events += metrics.Events
	}
	summary.Profit = summary.Revenue - summary.Cost
	if summary.Cost != 0 {
		summary.ROI = summary.Profit / summary.Cost * 100
	}

	total := len(reportRows)
	if groupBy != GroupByTrafficback && groupBy != GroupByHealth {
		totals, err := r.queryMetrics(
			ctx,
			groupedTotalsSQL(
				groupBy,
				query,
			),
		)
		if err != nil {
			return GroupedReport{}, err
		}
		if len(totals) > 0 {
			summary, total = totals[0].Metrics(), totals[0].TotalRows
		}
	}
	limit := query.Limit
	if limit <= 0 {
		limit = defaultReportLimit
	}
	return GroupedReport{
		Total:   total,
		Limit:   limit,
		Offset:  query.Offset,
		GroupBy: string(groupBy),
		Summary: summary,
		Rows:    reportRows,
		Filters: query.Filters(),
	}, nil
}

func (r *ClickHouseRepository) Daily(
	ctx context.Context,
	query Query,
) (
	GroupedReport,
	error,
) {
	query.OwnerID = scope.OwnerID(ctx)
	rows, err := r.queryMetrics(
		ctx,
		buildDailySQL(query),
	)
	if err != nil {
		return GroupedReport{}, err
	}

	reportRows := make(
		[]ReportRow,
		0,
		len(rows),
	)
	var summary Metrics
	for _, row := range rows {
		metrics := row.Metrics()
		reportRows = append(
			reportRows,
			ReportRow{
				ID:      row.ID,
				Name:    row.Name,
				Metrics: metrics,
			},
		)
		summary.Clicks += metrics.Clicks
		summary.Conversions += metrics.Conversions
		summary.Revenue += metrics.Revenue
		summary.Cost += metrics.Cost
	}
	summary.Profit = summary.Revenue - summary.Cost
	if summary.Cost != 0 {
		summary.ROI = summary.Profit / summary.Cost * 100
	}

	return GroupedReport{
		GroupBy: "day",
		Summary: summary,
		Rows:    reportRows,
		Filters: query.Filters(),
	}, nil
}

func (r *ClickHouseRepository) IngestionErrors(
	ctx context.Context,
	query IngestionErrorsQuery,
) (
	IngestionErrorsReport,
	error,
) {
	rows, err := r.queryIngestionErrors(
		ctx,
		buildIngestionErrorsSQL(query),
	)
	if err != nil {
		return IngestionErrorsReport{}, err
	}
	return IngestionErrorsReport{
		Rows:    rows,
		Filters: query.Filters(),
	}, nil
}

func (r *ClickHouseRepository) queryMetrics(
	ctx context.Context,
	sql string,
) (
	[]metricRow,
	error,
) {
	if strings.TrimSpace(r.endpoint) == "" {
		return nil, errors.New("clickhouse endpoint is required")
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		r.endpoint,
		strings.NewReader(sql),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set(
		"Content-Type",
		"text/plain; charset=utf-8",
	)

	res, err := r.httpClient.Do(req)
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

	rows, err := decodeMetricRows(body)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ClickHouseRepository) queryIngestionErrors(
	ctx context.Context,
	sql string,
) (
	[]IngestionErrorRow,
	error,
) {
	if strings.TrimSpace(r.endpoint) == "" {
		return nil, errors.New("clickhouse endpoint is required")
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		r.endpoint,
		strings.NewReader(sql),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set(
		"Content-Type",
		"text/plain; charset=utf-8",
	)

	res, err := r.httpClient.Do(req)
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

	rows, err := decodeIngestionErrorRows(body)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

type metricRow struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Clicks      int     `json:"clicks"`
	Conversions int     `json:"conversions"`
	Revenue     float64 `json:"revenue"`
	Cost        float64 `json:"cost"`
	Profit      float64 `json:"profit"`
	ROI         float64 `json:"roi"`
	Events      int     `json:"events"`
	TotalRows   int     `json:"total_rows"`
}

func (r *metricRow) UnmarshalJSON(payload []byte) error {
	var raw struct {
		ID          string          `json:"id"`
		Name        string          `json:"name"`
		Clicks      json.RawMessage `json:"clicks"`
		Conversions json.RawMessage `json:"conversions"`
		Revenue     json.RawMessage `json:"revenue"`
		Cost        json.RawMessage `json:"cost"`
		Profit      json.RawMessage `json:"profit"`
		ROI         json.RawMessage `json:"roi"`
		Events      json.RawMessage `json:"events"`
		TotalRows   json.RawMessage `json:"total_rows"`
	}
	if err := json.Unmarshal(
		payload,
		&raw,
	); err != nil {
		return err
	}

	totalRows, err := parseFlexibleInt(raw.TotalRows)
	if err != nil {
		return err
	}
	r.TotalRows = totalRows
	clicks, err := parseFlexibleInt(raw.Clicks)
	if err != nil {
		return err
	}
	conversions, err := parseFlexibleInt(raw.Conversions)
	if err != nil {
		return err
	}
	revenue, err := parseFlexibleFloat(raw.Revenue)
	if err != nil {
		return err
	}
	cost, err := parseFlexibleFloat(raw.Cost)
	if err != nil {
		return err
	}
	profit, err := parseFlexibleFloat(raw.Profit)
	if err != nil {
		return err
	}
	roi, err := parseFlexibleFloat(raw.ROI)
	if err != nil {
		return err
	}
	events, err := parseFlexibleInt(raw.Events)
	if err != nil {
		return err
	}

	*r = metricRow{
		TotalRows:   totalRows,
		ID:          raw.ID,
		Name:        raw.Name,
		Clicks:      clicks,
		Conversions: conversions,
		Revenue:     revenue,
		Cost:        cost,
		Profit:      profit,
		ROI:         roi,
		Events:      events,
	}
	return nil
}

func (r metricRow) Metrics() Metrics {
	return Metrics{
		Clicks:      r.Clicks,
		Conversions: r.Conversions,
		Revenue:     r.Revenue,
		Cost:        r.Cost,
		Profit:      r.Profit,
		ROI:         r.ROI,
		Events:      r.Events,
	}
}

func decodeMetricRows(body []byte) (
	[]metricRow,
	error,
) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	rows := []metricRow{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row metricRow
		if err := json.Unmarshal(
			[]byte(line),
			&row,
		); err != nil {
			return nil, err
		}
		rows = append(
			rows,
			row,
		)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func parseFlexibleInt(raw json.RawMessage) (
	int,
	error,
) {
	if len(raw) == 0 {
		return 0, nil
	}
	value, err := parseFlexibleFloat(raw)
	if err != nil {
		return 0, err
	}
	return int(value), nil
}

func parseFlexibleFloat(raw json.RawMessage) (
	float64,
	error,
) {
	if len(raw) == 0 {
		return 0, nil
	}
	var number float64
	if err := json.Unmarshal(
		raw,
		&number,
	); err == nil {
		return number, nil
	}

	var text string
	if err := json.Unmarshal(
		raw,
		&text,
	); err != nil {
		return 0, err
	}
	if strings.TrimSpace(text) == "" {
		return 0, nil
	}
	return strconv.ParseFloat(
		text,
		64,
	)
}

func decodeIngestionErrorRows(body []byte) (
	[]IngestionErrorRow,
	error,
) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	rows := []IngestionErrorRow{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row IngestionErrorRow
		if err := json.Unmarshal(
			[]byte(line),
			&row,
		); err != nil {
			return nil, err
		}
		rows = append(
			rows,
			row,
		)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func buildOverviewSQL(query Query) string {
	clickTable, clickCount := clickReportSource(
		query,
		false,
	)
	clickWhere := buildWhereClause(
		clickTable,
		query,
	)
	conversionWhere := buildWhereClause(
		conversionEventsTable,
		query,
	)
	return fmt.Sprintf(
		`SELECT clicks, conversions, revenue, cost, revenue - cost AS profit, if(cost = 0, 0, (revenue - cost) / cost * 100) AS roi
FROM (
	SELECT sum(clicks) AS clicks, sum(conversions) AS conversions, sum(revenue) AS revenue, sum(cost) AS cost
	FROM (
		SELECT %s AS clicks, 0 AS conversions, 0.0 AS revenue, sum(cost) AS cost
		FROM %s
		%s
		UNION ALL
		SELECT 0 AS clicks, count() AS conversions, sum(payout) AS revenue, 0.0 AS cost
		FROM %s
		%s
	)
)
FORMAT JSONEachRow`,
		clickCount,
		clickTable,
		clickWhere,
		conversionSource(
			query,
			"",
		),
		conversionWhere,
	)
}

func buildGroupedSQL(
	groupBy GroupBy,
	query Query,
) string {
	switch groupBy {
	case GroupByTrafficback:
		return buildEventGroupedSQL(
			trafficbackEventsTable,
			"reason",
			query,
		)
	case GroupByHealth:
		return buildEventGroupedSQL(
			destinationHealthEventsTable,
			"current",
			query,
		)
	}
	limit := query.Limit
	if limit <= 0 {
		limit = defaultReportLimit
	}
	return fmt.Sprintf(
		"%s\n%s\nORDER BY %s\nLIMIT %d OFFSET %d\nFORMAT JSONEachRow",
		groupedCoreSQL(
			groupBy,
			query,
		),
		groupedRowFilters(query),
		reportRowOrder(query),
		limit,
		query.Offset,
	)
}

func groupedCoreSQL(
	groupBy GroupBy,
	query Query,
) string {
	column := reportGroupColumn(groupBy)
	clickTable, clickCount := clickReportSource(
		query,
		false,
	)
	clickWhere := buildWhereClause(
		clickTable,
		query,
	)
	conversionWhere := buildWhereClause(
		conversionEventsTable,
		query,
	)
	return fmt.Sprintf(
		`SELECT id, if(id = '', 'Unknown', id) AS name, clicks, conversions, revenue, cost,
revenue - cost AS profit, if(cost = 0, 0, (revenue - cost) / cost * 100) AS roi
FROM (
    SELECT id, sum(clicks) AS clicks, sum(conversions) AS conversions, sum(revenue) AS revenue, sum(cost) AS cost
    FROM (
        SELECT %s AS id, %s AS clicks, 0 AS conversions, 0.0 AS revenue, sum(cost) AS cost
        FROM %s
        %s
        GROUP BY id
        UNION ALL
        SELECT %s AS id, 0 AS clicks, count() AS conversions, sum(payout) AS revenue, 0.0 AS cost
        FROM %s
        %s
        GROUP BY id
    )
    GROUP BY id
)`,
		reportDimensionExpression(
			clickTable,
			column,
		),
		clickCount,
		clickTable,
		clickWhere,
		column,
		conversionSource(
			query,
			groupBy,
		),
		conversionWhere,
	)
}

func groupedTotalsSQL(
	groupBy GroupBy,
	query Query,
) string {
	return fmt.Sprintf(
		`SELECT count() AS total_rows, sum(clicks) AS clicks, sum(conversions) AS conversions,
sum(revenue) AS revenue, sum(cost) AS cost, revenue - cost AS profit,
if(cost = 0, 0, profit / cost * 100) AS roi
FROM (%s %s) FORMAT JSONEachRow`,
		groupedCoreSQL(
			groupBy,
			query,
		),
		groupedRowFilters(query),
	)
}

func conversionSource(
	query Query,
	group GroupBy,
) string {
	if !isReportDimension(string(group)) && !hasDimensionFilters(query) {
		return conversionReportSource
	}
	// Bound the lookup to conversion click IDs in the requested workspace and
	// period. Do not scan raw click history or duplicate conversion revenue.
	identityQuery := Query{OwnerID: query.OwnerID, From: query.From, To: query.To}
	where := buildWhereClause(
		conversionEventsTable,
		identityQuery,
	)
	lookupWhere := ""
	if query.OwnerID != "" {
		lookupWhere = "owner_id = " + quoteString(query.OwnerID) + " AND "
	}
	fields := make(
		[]string,
		0,
		len(reportDimensions),
	)
	aggregates := make(
		[]string,
		0,
		len(reportDimensions),
	)
	for _, field := range reportDimensions {
		fields = append(
			fields,
			"d."+field+" AS "+field,
		)
		aggregates = append(
			aggregates,
			"any("+field+") AS "+field,
		)
	}
	return fmt.Sprintf(
		`(SELECT c.*, %s FROM %s AS c
LEFT ANY JOIN (SELECT owner_id, click_id, %s FROM click_attribution_lookup
WHERE %sclick_id IN (SELECT click_id FROM conversion_events %s)
GROUP BY owner_id, click_id) AS d
ON c.owner_id = d.owner_id AND c.click_id = d.click_id)`,
		strings.Join(
			fields,
			", ",
		),
		conversionReportSource,
		strings.Join(
			aggregates,
			", ",
		),
		lookupWhere,
		where,
	)
}

func buildDailySQL(query Query) string {
	clickTable, clickCount := clickReportSource(
		query,
		true,
	)
	clickWhere := buildWhereClause(
		clickTable,
		query,
	)
	conversionWhere := buildWhereClause(
		conversionEventsTable,
		query,
	)
	timezone := quoteString(query.Timezone)
	return fmt.Sprintf(
		`SELECT id, id AS name, clicks, conversions, revenue, cost, revenue - cost AS profit, if(cost = 0, 0, (revenue - cost) / cost * 100) AS roi
FROM (
	SELECT id, sum(clicks) AS clicks, sum(conversions) AS conversions, sum(revenue) AS revenue, sum(cost) AS cost
	FROM (
		SELECT toString(toDate(toTimeZone(created_at, %s))) AS id, %s AS clicks, 0 AS conversions, 0.0 AS revenue, sum(cost) AS cost
		FROM %s
		%s
		GROUP BY id
		UNION ALL
		SELECT toString(toDate(toTimeZone(created_at, %s))) AS id, 0 AS clicks, count() AS conversions, sum(payout) AS revenue, 0.0 AS cost
		FROM %s
		%s
		GROUP BY id
	)
	GROUP BY id
)
ORDER BY id ASC
FORMAT JSONEachRow`,
		timezone,
		clickCount,
		clickTable,
		clickWhere,
		timezone,
		conversionSource(
			query,
			"",
		),
		conversionWhere,
	)
}

func clickReportSource(
	query Query,
	daily bool,
) (string, string) {
	from, to := query.From.UTC(), query.To.UTC()
	// Daily buckets are UTC. Other timezones use events for the daily chart so
	// half-hour offsets and daylight saving boundaries remain exact.
	if daily && query.Timezone != "" && query.Timezone != "UTC" && query.Timezone != "Etc/UTC" {
		return clickEventsTable, "count()"
	}
	wholeStart := from.IsZero() || (from.Minute() == 0 && from.Second() == 0 && from.Nanosecond() == 0)
	wholeEnd := to.IsZero() || (to.Minute() == 59 && to.Second() == 59)
	if !wholeStart || !wholeEnd {
		return clickEventsTable, "count()"
	}
	wholeDayStart := from.IsZero() || from.Hour() == 0
	wholeDayEnd := to.IsZero() || to.Hour() == 23
	if wholeDayStart && wholeDayEnd {
		return clickStatsDayTable, "sum(clicks)"
	}
	return clickStatsHourTable, "sum(clicks)"
}

func buildIngestionErrorsSQL(query IngestionErrorsQuery) string {
	where := buildIngestionErrorsWhereClause(query)
	return fmt.Sprintf(
		`SELECT observed_at, topic, error, raw_message
FROM %s
%s
ORDER BY observed_at DESC
LIMIT %d
FORMAT JSONEachRow`,
		kafkaIngestionErrorsTable,
		where,
		query.Limit,
	)
}

func buildIngestionErrorsWhereClause(query IngestionErrorsQuery) string {
	conditions := make(
		[]string,
		0,
	)
	if !query.From.IsZero() {
		conditions = append(
			conditions,
			fmt.Sprintf(
				"observed_at >= toDateTime('%s')",
				query.From.UTC().Format(time.DateTime),
			),
		)
	}
	if !query.To.IsZero() {
		conditions = append(
			conditions,
			fmt.Sprintf(
				"observed_at <= toDateTime('%s')",
				query.To.UTC().Format(time.DateTime),
			),
		)
	}
	if query.Topic != "" {
		conditions = append(
			conditions,
			fmt.Sprintf(
				"topic = '%s'",
				strings.ReplaceAll(
					query.Topic,
					"'",
					"''",
				),
			),
		)
	}
	if len(conditions) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(
		conditions,
		" AND ",
	)
}

func buildEventGroupedSQL(
	table,
	column string,
	query Query,
) string {
	where := appendWhereCondition(
		buildWhereClause(
			table,
			query,
		),
		column+" != ''",
	)
	return fmt.Sprintf(
		`SELECT %s AS id, %s AS name, 0 AS clicks, 0 AS conversions, 0.0 AS revenue, 0.0 AS cost, 0.0 AS profit, 0.0 AS roi, count() AS events
FROM %s
%s
GROUP BY id
ORDER BY events DESC
LIMIT %d
FORMAT JSONEachRow`,
		column,
		column,
		table,
		where,
		defaultReportLimit,
	)
}

func reportGroupColumn(groupBy GroupBy) string {
	switch groupBy {
	case GroupByCampaign:
		return "campaign_id"
	case GroupByStream:
		return "stream_id"
	case GroupByDestination:
		return "destination_id"
	case GroupBySource:
		return "source_id"
	default:
		if isReportDimension(string(groupBy)) {
			return string(groupBy)
		}
		return "campaign_id"
	}
}

func buildWhereClause(
	table string,
	query Query,
) string {
	conditions := make(
		[]string,
		0,
		6,
	)
	if query.OwnerID != "" {
		conditions = append(
			conditions,
			"owner_id = "+quoteString(query.OwnerID),
		)
	}
	if !query.From.IsZero() {
		conditions = append(
			conditions,
			"created_at >= "+quoteTime(reportLowerBound(query.From)),
		)
	}
	if !query.To.IsZero() {
		conditions = append(
			conditions,
			"created_at <= "+quoteTime(query.To.UTC()),
		)
	}
	if supportsFilter(
		table,
		"campaign_id",
	) && query.CampaignID != "" {
		conditions = append(
			conditions,
			"campaign_id = "+quoteString(query.CampaignID),
		)
	}
	if supportsFilter(
		table,
		"stream_id",
	) && query.StreamID != "" {
		conditions = append(
			conditions,
			"stream_id = "+quoteString(query.StreamID),
		)
	}
	if supportsFilter(
		table,
		"destination_id",
	) && query.DestinationID != "" {
		conditions = append(
			conditions,
			"destination_id = "+quoteString(query.DestinationID),
		)
	}
	if supportsFilter(
		table,
		"source_id",
	) && query.SourceID != "" {
		conditions = append(
			conditions,
			"source_id = "+quoteString(query.SourceID),
		)
	}
	for _, field := range reportDimensions {
		if value, exists := query.Dimensions[field]; exists && supportsFilter(
			table,
			field,
		) {
			conditions = append(
				conditions,
				reportDimensionExpression(
					table,
					field,
				)+" = "+quoteString(value),
			)
		}
	}
	for _, field := range query.EmptyFields {
		if supportsFilter(
			table,
			field,
		) {
			conditions = append(
				conditions,
				reportDimensionExpression(
					table,
					field,
				)+" = ''",
			)
		}
	}
	if len(conditions) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(
		conditions,
		" AND ",
	)
}

func supportsFilter(
	table,
	field string,
) bool {
	switch table {
	case clickEventsTable, clickStatsDayTable, clickStatsHourTable:
		return isReportDimension(field) || field == "campaign_id" ||
			field == "stream_id" ||
			field == "destination_id" ||
			field == "source_id"
	case conversionEventsTable:
		return isReportDimension(field) || field == "campaign_id" ||
			field == "stream_id" ||
			field == "destination_id" ||
			field == "source_id"
	case trafficbackEventsTable:
		return field == "campaign_id" ||
			field == "stream_id" ||
			field == "destination_id"
	case destinationHealthEventsTable:
		return field == "destination_id"
	default:
		return false
	}
}

func reportLowerBound(value time.Time) time.Time {
	if value.Nanosecond() > 0 {
		return value.UTC().Truncate(time.Second).Add(time.Second)
	}
	return value.UTC()
}

func quoteTime(value time.Time) string {
	return quoteString(value.UTC().Format(time.DateTime))
}

func appendWhereCondition(
	where,
	condition string,
) string {
	if where == "" {
		return "WHERE " + condition
	}
	return where + " AND " + condition
}

func quoteString(value string) string {
	value = strings.ReplaceAll(
		value,
		"\\",
		"\\\\",
	)
	return "'" + strings.ReplaceAll(
		value,
		"'",
		"''",
	) + "'"
}
