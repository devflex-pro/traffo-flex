package availability

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestClickHouseCapCheckerExceeded(t *testing.T) {
	doer := &capDoer{
		body: `{"value": "101"}`,
	}
	checker := NewClickHouseCapCheckerWithDoer(
		"http://clickhouse:8123",
		doer,
		time.Second,
	)
	exceeded, err := checker.Exceeded(
		context.Background(),
		models.Destination{
			ID: "dst_1",
			Caps: models.DestinationCaps{
				Enabled: true,
				Rules: []models.DestinationCapRule{
					{
						Metric:      models.CapMetricClicks,
						WindowHours: 24,
						Limit:       100,
					},
				},
			},
		},
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf(
			"Exceeded returned error: %v",
			err,
		)
	}
	if !exceeded {
		t.Fatal("cap should be exceeded")
	}
	if !strings.Contains(
		doer.sql,
		"click_stats_1h",
	) {
		t.Fatalf(
			"query = %q, want click_stats_1h",
			doer.sql,
		)
	}
	if !strings.Contains(doer.sql, "sum(clicks)") {
		t.Fatalf("click cap must sum the rollup count: %s", doer.sql)
	}
	if !strings.Contains(doer.sql, "toStartOfHour(now() - INTERVAL 24 HOUR) + INTERVAL 1 HOUR") {
		t.Fatalf("click cap must use full hours and the partial boundary hour: %s", doer.sql)
	}
}

func TestBuildCapSQLUsesConversionTableForRevenue(t *testing.T) {
	sql := buildCapSQL(
		"dst_1",
		models.DestinationCapRule{
			Metric:      models.CapMetricRevenue,
			WindowHours: 24,
			Limit:       100,
		},
	)
	if !strings.Contains(
		sql,
		"attributed_conversion_events",
	) {
		t.Fatalf(
			"sql = %q, want attributed_conversion_events",
			sql,
		)
	}
	if !strings.Contains(
		sql,
		"sum(payout)",
	) {
		t.Fatalf(
			"sql = %q, want sum(payout)",
			sql,
		)
	}
	if strings.Contains(sql, "FROM click_events GROUP BY click_id") {
		t.Fatalf("revenue cap scans all clicks: %s", sql)
	}
}

func TestBuildROISQLAttributesByClickID(t *testing.T) {
	sql := buildROISQL([]string{"dst_1"}, 24)
	if !strings.Contains(sql, "FROM attributed_conversion_events") {
		t.Fatalf("ROI must use attributed conversions: %s", sql)
	}
	if !strings.Contains(sql, "FROM click_stats_1h") || !strings.Contains(sql, "FROM click_events") {
		t.Fatalf("ROI must use the hourly click rollup: %s", sql)
	}
	if !strings.Contains(sql, "toStartOfHour(now() - INTERVAL 24 HOUR) + INTERVAL 1 HOUR") {
		t.Fatalf("ROI must include complete hours and the exact raw boundary: %s", sql)
	}
}

func TestClickHousePolicyStoreReadsUsedDestinations(t *testing.T) {
	doer := &capDoer{
		body: `{"destination_id":"dst_seen"}` + "\n",
	}
	checker := NewClickHouseCapCheckerWithDoer(
		"http://clickhouse:8123",
		doer,
		time.Second,
	)
	used, err := checker.UsedDestinations(
		context.Background(),
		"source_click_id",
		"user_1",
		168,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf(
			"UsedDestinations returned error: %v",
			err,
		)
	}
	if _, ok := used["dst_seen"]; !ok {
		t.Fatalf(
			"used destinations = %#v, want dst_seen",
			used,
		)
	}
	if !strings.Contains(
		doer.sql,
		"source_click_id",
	) {
		t.Fatalf(
			"query = %q, want source_click_id",
			doer.sql,
		)
	}
}

func TestDecodeROIRankingSortsByROI(t *testing.T) {
	ranked, err := decodeROIRanking(
		[]byte(`{"destination_id":"dst_a","clicks":100,"conversions":10,"revenue":200,"cost":100}
{"destination_id":"dst_b","clicks":100,"conversions":10,"revenue":160,"cost":100}`),
		10,
	)
	if err != nil {
		t.Fatalf(
			"decodeROIRanking returned error: %v",
			err,
		)
	}
	if len(ranked) != 2 || ranked[0] != "dst_a" || ranked[1] != "dst_b" {
		t.Fatalf(
			"ranked = %#v, want dst_a,dst_b",
			ranked,
		)
	}
}

type capDoer struct {
	body string
	sql  string
}

func (d *capDoer) Do(req *http.Request) (
	*http.Response,
	error,
) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	d.sql = string(body)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(d.body)),
	}, nil
}
