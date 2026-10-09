package reports

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devflex/traffoflex/apps/api-service/internal/scope"
)

func TestDimensionalReportsOnClickHouse(t *testing.T) {
	endpoint := os.Getenv("ANALYTICS_TEST_CLICKHOUSE_URL")
	if endpoint == "" {
		t.Skip("set ANALYTICS_TEST_CLICKHOUSE_URL to the isolated report test database")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Query().Get("database") != "traffoflex_report_test" || (parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost") {
		t.Fatal("integration tests require a local traffoflex_report_test database")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	execute := func(sql string) string {
		t.Helper()
		response, err := client.Post(
			endpoint,
			"text/plain",
			strings.NewReader(sql),
		)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK {
			t.Fatalf(
				"ClickHouse status %d: %s; read=%v close=%v",
				response.StatusCode,
				body,
				readErr,
				closeErr,
			)
		}
		return string(body)
	}
	for _, table := range []string{"click_events", "click_stats_1h", "click_stats_1d", "click_attribution_lookup", "conversion_events", "attributed_conversion_events"} {
		execute("TRUNCATE TABLE " + table)
	}
	execute(`INSERT INTO click_events (created_at, owner_id, click_id, campaign_id, stream_id, destination_id, source_id, geo_country, device_type, os, browser, sub1, sub2, sub3, sub4, cost, query) VALUES
('2026-01-01 00:00:00','owner','click_a','cmp_1','str_1','dst_1','src_1','US','mobile','Android','Chrome','','','','',0.1,'{"zone_id":"z1","publisher_id":"p1","site_id":"site_a","creative_id":"cr1","carrier":"Carrier A"}'),
('2026-01-01 12:30:00','owner','click_b','cmp_1','str_1','dst_1','src_1','US','desktop','Windows','Firefox','z2','p1','site_b','cr2',0.2,'{}'),
('2026-01-02 01:00:00','owner','click_c','cmp_1','str_1','dst_1','src_1','DE','mobile','Android','Chrome','z3','p2','site_c','cr3',0.3,'{}'),
('2026-01-02 02:00:00','owner','click_d','cmp_1','str_1','dst_1','src_1','','','','','z4','','','',0.4,'{}'),
('2026-01-01 00:00:00','other','click_a','cmp_other','str_other','dst_other','src_other','US','mobile','Android','Chrome','private','private','private','private',9,'{}')`)
	execute(`INSERT INTO conversion_events (created_at, updated_at, owner_id, conversion_id, click_id, transaction_id, payout) VALUES
('2026-01-01 02:00:00','2026-01-01 02:00:00','owner','conv_a','click_a','tx_a',1.25),
('2026-01-01 02:00:00','2026-01-01 02:00:00','owner','conv_a','click_a','tx_a',1.25),
('2026-01-01 13:00:00','2026-01-01 13:00:00','owner','conv_b','click_b','tx_b',0.05),
('2026-01-02 03:00:00','2026-01-02 03:00:00','owner','conv_orphan','missing','tx_orphan',0.7),
('2026-01-01 02:00:00','2026-01-01 02:00:00','other','conv_a','click_a','tx_a',99)`)
	execute(`INSERT INTO attributed_conversion_events (created_at, attributed_at, owner_id, conversion_id, click_id, campaign_id, stream_id, destination_id, source_id, payout) VALUES
('2026-01-01 02:00:00','2026-01-01 02:00:00','owner','conv_a','click_a','cmp_1','str_1','dst_1','src_1',1.25),
('2026-01-01 02:00:00','2026-01-01 02:00:00','owner','conv_a','click_a','cmp_1','str_1','dst_1','src_1',1.25),
('2026-01-01 13:00:00','2026-01-01 13:00:00','owner','conv_b','click_b','cmp_1','str_1','dst_1','src_1',0.05),
('2026-01-01 02:00:00','2026-01-01 02:00:00','other','conv_a','click_a','cmp_other','str_other','dst_other','src_other',99)`)
	ctx := scope.WithValue(
		context.Background(),
		scope.Value{ActorID: "owner", OwnerID: "owner"},
	)
	repo := NewClickHouseRepository(endpoint)
	query := Query{From: mustDate(
		t,
		"2026-01-01",
	), To: mustDate(
		t,
		"2026-01-02",
	).Add(24*time.Hour - time.Second), Timezone: "UTC", Limit: 100}
	near := func(
		got,
		want float64,
	) bool {
		return math.Abs(got-want) < 1e-9
	}
	all, err := repo.Grouped(
		ctx,
		GroupBy("geo_country"),
		query,
	)
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 3 || all.Summary.Clicks != 4 || all.Summary.Conversions != 3 || !near(
		all.Summary.Cost,
		1,
	) || !near(
		all.Summary.Revenue,
		2,
	) {
		t.Fatalf(
			"country totals/dedup/isolation: %+v",
			all,
		)
	}
	byID := map[string]Metrics{}
	for _, row := range all.Rows {
		byID[row.ID] = row.Metrics
	}
	if byID["US"].Clicks != 2 || byID["US"].Conversions != 2 || !near(
		byID["US"].Revenue,
		1.3,
	) || byID[""].Clicks != 1 || !near(
		byID[""].Revenue,
		0.7,
	) {
		t.Fatalf(
			"incorrect dimensional or unknown attribution: %+v",
			byID,
		)
	}
	query.Dimensions = map[string]string{"geo_country": "US", "device_type": "mobile"}
	segment, err := repo.Grouped(
		ctx,
		GroupBy("site_id"),
		query,
	)
	if err != nil {
		t.Fatal(err)
	}
	if segment.Total != 1 || segment.Rows[0].ID != "site_a" || segment.Summary.Clicks != 1 || segment.Summary.Conversions != 1 || !near(
		segment.Summary.Cost,
		0.1,
	) || !near(
		segment.Summary.Revenue,
		1.25,
	) {
		t.Fatalf(
			"country/device/site drill: %+v",
			segment,
		)
	}
	overview, err := repo.Overview(
		ctx,
		query,
	)
	if err != nil {
		t.Fatal(err)
	}
	if overview != segment.Summary {
		t.Fatalf(
			"segment summary differs: %+v versus %+v",
			overview,
			segment.Summary,
		)
	}
	query.Dimensions = nil
	query.Limit, query.Offset, query.Sort, query.Order = 1, 1, "profit", "desc"
	page, err := repo.Grouped(
		ctx,
		GroupBy("geo_country"),
		query,
	)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Rows) != 1 || page.Rows[0].ID != "" || page.Summary.Clicks != 4 || !near(
		page.Summary.Revenue,
		2,
	) {
		t.Fatalf(
			"pagination/sorting/global totals: %+v",
			page,
		)
	}
	query.Offset, query.Limit, query.Profit = 0, 100, "negative"
	losing, err := repo.Grouped(
		ctx,
		GroupBy("zone_id"),
		query,
	)
	if err != nil {
		t.Fatal(err)
	}
	if losing.Total != 3 || losing.Summary.Clicks != 3 || losing.Summary.Conversions != 1 || !near(
		losing.Summary.Cost,
		0.9,
	) || !near(
		losing.Summary.Revenue,
		0.05,
	) {
		t.Fatalf(
			"losing zones/legacy named aliases: %+v",
			losing,
		)
	}
	query.Profit = ""
	query.EmptyFields = []string{"geo_country"}
	unknown, err := repo.Grouped(
		ctx,
		GroupBy("geo_country"),
		query,
	)
	if err != nil {
		t.Fatal(err)
	}
	if unknown.Total != 1 || unknown.Rows[0].ID != "" || unknown.Summary.Clicks != 1 || unknown.Summary.Conversions != 1 {
		t.Fatalf(
			"unknown segment: %+v",
			unknown,
		)
	}
	query.EmptyFields = nil
	query.From = mustDate(
		t,
		"2026-01-01",
	).Add(12*time.Hour + 30*time.Minute + time.Second)
	query.To = mustDate(
		t,
		"2026-01-01",
	).Add(24*time.Hour - time.Second)
	partial, err := repo.Grouped(
		ctx,
		GroupBy("site_id"),
		query,
	)
	if err != nil {
		t.Fatal(err)
	}
	if partial.Summary.Clicks != 0 || partial.Summary.Conversions != 1 || !near(
		partial.Summary.Revenue,
		0.05,
	) {
		t.Fatalf(
			"partial hour boundaries: %+v",
			partial,
		)
	}
	query.From = mustDate(
		t,
		"2025-12-31",
	).Add(18*time.Hour + 30*time.Minute)
	query.To = mustDate(
		t,
		"2026-01-01",
	).Add(18*time.Hour + 30*time.Minute - time.Second)
	query.Timezone = "Asia/Kolkata"
	daily, err := repo.Daily(
		ctx,
		query,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(daily.Rows) != 1 || daily.Rows[0].ID != "2026-01-01" || daily.Summary.Clicks != 2 || !near(
		daily.Summary.Revenue,
		1.3,
	) {
		t.Fatalf(
			"half-hour timezone: %+v",
			daily,
		)
	}
	query.Dimensions = map[string]string{"geo_country": "US'\\ OR 1=1 --"}
	escaped, err := repo.Grouped(
		ctx,
		GroupBy("geo_country"),
		query,
	)
	if err != nil {
		t.Fatal(err)
	}
	if escaped.Total != 0 || escaped.Summary.Clicks != 0 || escaped.Summary.Conversions != 0 {
		t.Fatalf(
			"dimension escaped incorrectly: %+v",
			escaped,
		)
	}
	execute("SYSTEM FLUSH LOGS")
}
