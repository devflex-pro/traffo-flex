package postbacklogs

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseQueryReadsPagination(t *testing.T) {
	req := httptest.NewRequest(
		"GET",
		"/postback-logs?network_id=net_1&limit=25&offset=50",
		nil,
	)
	query, err := ParseQuery(req)
	if err != nil {
		t.Fatalf(
			"ParseQuery returned error: %v",
			err,
		)
	}
	if query.NetworkID != "net_1" {
		t.Fatalf(
			"network_id = %q, want net_1",
			query.NetworkID,
		)
	}
	if query.Limit != 25 {
		t.Fatalf(
			"limit = %d, want 25",
			query.Limit,
		)
	}
	if query.Offset != 50 {
		t.Fatalf(
			"offset = %d, want 50",
			query.Offset,
		)
	}
}

func TestParseQueryReadsIDPeriodAndOrder(t *testing.T) {
	req := httptest.NewRequest(
		"GET",
		"/postback-logs?id=clk_1&from=2026-10-08T03:00:00%2B03:00&to=2026-10-08T23:59:59.999Z&order=asc",
		nil,
	)
	query, err := ParseQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	if query.ID != "clk_1" || query.Order != "asc" || query.From.Format(time.RFC3339) != "2026-10-08T00:00:00Z" || query.Filters()["to"] != "2026-10-08T23:59:59.999Z" {
		t.Fatalf(
			"unexpected filters: %v",
			query.Filters(),
		)
	}
}

func TestParseQueryRejectsInvalidLogFilters(t *testing.T) {
	for _, raw := range []string{"from=yesterday", "to=2026-10-08", "from=2026-10-09T00:00:00Z&to=2026-10-08T00:00:00Z", "order=arbitrary", "id=%24where"} {
		req := httptest.NewRequest(
			"GET",
			"/postback-logs?"+raw,
			nil,
		)
		if _, err := ParseQuery(req); err == nil {
			t.Fatalf(
				"accepted invalid filters %q",
				raw,
			)
		}
	}
}

func TestParseQueryRejectsInvalidOffset(t *testing.T) {
	req := httptest.NewRequest(
		"GET",
		"/postback-logs?offset=-1",
		nil,
	)
	if _, err := ParseQuery(req); err == nil {
		t.Fatal("ParseQuery expected error")
	}
}
