package reports

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/devflex/traffoflex/apps/api-service/internal/scope"
)

type captureDoer struct {
	status int
	body   string
	query  string
}

func (d *captureDoer) Do(req *http.Request) (
	*http.Response,
	error,
) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	if closeErr := req.Body.Close(); closeErr != nil {
		return nil, closeErr
	}
	d.query = string(body)
	status := d.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(d.body)),
	}, nil
}

func TestClickHouseOverviewQueriesMetrics(t *testing.T) {
	doer := &captureDoer{
		body: `{"clicks":10,"conversions":2,"revenue":12.5,"cost":2.5,"profit":10,"roi":400}` + "\n",
	}
	repo := NewClickHouseRepositoryWithDoer(
		"http://clickhouse:8123",
		doer,
	)

	metrics, err := repo.Overview(
		context.Background(),
		Query{
			From:       mustDate(t, "2026-01-01"),
			To:         mustDate(t, "2026-01-31").Add(24*time.Hour - time.Second),
			CampaignID: "cmp_1",
		},
	)
	if err != nil {
		t.Fatalf(
			"overview: %v",
			err,
		)
	}
	if metrics.Clicks != 10 || metrics.Conversions != 2 {
		t.Fatalf(
			"metrics = %+v, want clicks=10 conversions=2",
			metrics,
		)
	}
	for _, want := range []string{
		"FROM click_stats_1h",
		"sum(clicks) AS clicks",
		"FROM conversion_events",
		"FROM attributed_conversion_events",
		"GROUP BY owner_id, conversion_id",
		"ON c.owner_id = k.owner_id AND c.conversion_id = k.conversion_id",
		"campaign_id = 'cmp_1'",
		"FORMAT JSONEachRow",
	} {
		if !strings.Contains(
			doer.query,
			want,
		) {
			t.Fatalf(
				"query does not contain %q:\n%s",
				want,
				doer.query,
			)
		}
	}
	if strings.Contains(doer.query, "FROM click_events GROUP BY click_id") {
		t.Fatalf("report scans all clicks for attribution: %s", doer.query)
	}
}

func TestClickHouseOverviewScopesBothClicksAndConversions(t *testing.T) {
	doer := &captureDoer{
		body: `{"clicks":0,"conversions":0,"revenue":0,"cost":0,"profit":0,"roi":0}` + "\n",
	}
	repo := NewClickHouseRepositoryWithDoer(
		"http://clickhouse:8123",
		doer,
	)
	ctx := scope.WithValue(
		context.Background(),
		scope.Value{ActorID: "usr_admin", OwnerID: "usr_owner"},
	)
	if _, err := repo.Overview(
		ctx,
		Query{},
	); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(
		doer.query,
		"owner_id = 'usr_owner'",
	); got != 2 {
		t.Fatalf(
			"owner filters = %d, want 2: %s",
			got,
			doer.query,
		)
	}
}

func TestClickReportSourcePreservesTimeBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		from  string
		to    string
		daily bool
		want  string
		count string
	}{
		{name: "all time", want: clickStatsHourTable, count: "sum(clicks)"},
		{name: "whole hours", from: "2026-01-01T00:00:00Z", to: "2026-01-01T23:59:59Z", want: clickStatsHourTable, count: "sum(clicks)"},
		{name: "whole minutes", from: "2026-01-01T00:01:00Z", to: "2026-01-01T00:02:59Z", want: clickStatsMinuteTable, count: "sum(clicks)"},
		{name: "daily timezone", from: "2026-01-01T00:00:00Z", to: "2026-01-01T23:59:59Z", daily: true, want: clickStatsMinuteTable, count: "sum(clicks)"},
		{name: "partial start", from: "2026-01-01T00:00:30Z", to: "2026-01-01T23:59:59Z", want: clickStatsHourTable, count: "sum(clicks)"},
		{name: "partial end", from: "2026-01-01T00:00:00Z", to: "2026-01-01T12:30:00Z", want: clickStatsMinuteTable, count: "sum(clicks)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var query Query
			if test.from != "" {
				var err error
				query.From, err = time.Parse(time.RFC3339, test.from)
				if err != nil {
					t.Fatal(err)
				}
			}
			if test.to != "" {
				var err error
				query.To, err = time.Parse(time.RFC3339, test.to)
				if err != nil {
					t.Fatal(err)
				}
			}
			table, count := clickReportSource(query, test.daily)
			if table != test.want {
				t.Fatalf("table = %s, want %s", table, test.want)
			}
			if count != test.count {
				t.Fatalf("count = %s, want %s", count, test.count)
			}
		})
	}
}

func TestReportTimeBoundsCoverWholeMinutesForClicksAndConversions(t *testing.T) {
	sql := buildOverviewSQL(Query{
		From: time.Date(2026, 1, 1, 0, 0, 30, 0, time.UTC),
		To:   time.Date(2026, 1, 1, 12, 30, 0, 0, time.UTC),
	})
	if !strings.Contains(sql, "FROM click_stats_1m") {
		t.Fatalf("report must use minute aggregate: %s", sql)
	}
	for _, want := range []string{
		"created_at >= '2026-01-01 00:00:00'",
		"created_at <= '2026-01-01 12:30:59'",
	} {
		if strings.Count(sql, want) != 2 {
			t.Fatalf("expected same minute boundary for clicks and conversions, %q in %s", want, sql)
		}
	}
}

func TestClickHouseGroupedCampaignReport(t *testing.T) {
	doer := &captureDoer{
		body: `{"id":"cmp_1","name":"Campaign","clicks":10,"conversions":2,"revenue":12.5,"cost":2.5,"profit":10,"roi":400}` + "\n",
	}
	repo := NewClickHouseRepositoryWithDoer(
		"http://clickhouse:8123",
		doer,
	)

	report, err := repo.Grouped(
		context.Background(),
		GroupByCampaign,
		Query{SourceID: "src_1"},
	)
	if err != nil {
		t.Fatalf(
			"grouped: %v",
			err,
		)
	}
	if report.GroupBy != "campaign" {
		t.Fatalf(
			"group_by = %q, want campaign",
			report.GroupBy,
		)
	}
	if len(report.Rows) != 1 {
		t.Fatalf(
			"rows len = %d, want 1",
			len(report.Rows),
		)
	}
	if !strings.Contains(
		doer.query,
		"source_id = 'src_1'",
	) {
		t.Fatalf(
			"query does not contain source filter:\n%s",
			doer.query,
		)
	}
	if strings.Contains(
		doer.query,
		";",
	) {
		t.Fatalf(
			"query contains statement separator:\n%s",
			doer.query,
		)
	}
}

func TestClickHouseDecodesQuotedMetricNumbers(t *testing.T) {
	doer := &captureDoer{
		body: `{"clicks":"10","conversions":"2","revenue":"12.5","cost":"2.5","profit":"10","roi":"400","events":"0"}` + "\n",
	}
	repo := NewClickHouseRepositoryWithDoer(
		"http://clickhouse:8123",
		doer,
	)

	metrics, err := repo.Overview(
		context.Background(),
		Query{},
	)
	if err != nil {
		t.Fatalf(
			"overview: %v",
			err,
		)
	}
	if metrics.Clicks != 10 || metrics.Revenue != 12.5 {
		t.Fatalf(
			"metrics = %+v, want quoted numbers decoded",
			metrics,
		)
	}
}

func TestClickHouseGroupedHealthReportUsesEventCount(t *testing.T) {
	doer := &captureDoer{
		body: `{"id":"healthy","name":"healthy","events":3}` + "\n",
	}
	repo := NewClickHouseRepositoryWithDoer(
		"http://clickhouse:8123",
		doer,
	)

	report, err := repo.Grouped(
		context.Background(),
		GroupByHealth,
		Query{DestinationID: "dst_1"},
	)
	if err != nil {
		t.Fatalf(
			"grouped health: %v",
			err,
		)
	}
	if report.Summary.Events != 3 {
		t.Fatalf(
			"events = %d, want 3",
			report.Summary.Events,
		)
	}
	for _, want := range []string{
		"FROM destination_health_events",
		"destination_id = 'dst_1'",
		"current != ''",
	} {
		if !strings.Contains(
			doer.query,
			want,
		) {
			t.Fatalf(
				"query does not contain %q:\n%s",
				want,
				doer.query,
			)
		}
	}
}

func TestClickHouseReturnsStatusError(t *testing.T) {
	repo := NewClickHouseRepositoryWithDoer(
		"http://clickhouse:8123",
		&captureDoer{
			status: http.StatusInternalServerError,
			body:   "boom",
		},
	)

	_, err := repo.Overview(
		context.Background(),
		Query{},
	)
	if err == nil {
		t.Fatal("expected error")
	}
}

func mustDate(
	t *testing.T,
	raw string,
) time.Time {
	t.Helper()
	parsed, err := time.Parse(
		time.DateOnly,
		raw,
	)
	if err != nil {
		t.Fatalf(
			"parse date: %v",
			err,
		)
	}
	return parsed
}
