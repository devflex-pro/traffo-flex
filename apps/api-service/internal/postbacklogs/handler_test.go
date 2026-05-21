package postbacklogs

import (
	"net/http/httptest"
	"testing"
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
