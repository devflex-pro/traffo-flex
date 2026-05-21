package reports

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
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
			To:         mustDate(t, "2026-01-31"),
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
		"FROM click_events",
		"FROM conversion_events",
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
