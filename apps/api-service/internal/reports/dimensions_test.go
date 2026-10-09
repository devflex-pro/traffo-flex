package reports

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReportDimensionQueryValidation(t *testing.T) {
	for _, raw := range []string{"group_by=zone_id%3BDROP+TABLE+click_events", "sort=profit%3BDROP+TABLE+click_events", "order=random", "limit=501", "limit=0", "offset=-1", "min_clicks=-1", "profit=maybe", "empty=raw_query", "geo_country=US%00", "timezone=invalid/timezone", "from=2026-03-09&to=2026-03-08"} {
		t.Run(raw, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodGet,
				"/reports/grouped?"+raw,
				nil,
			)
			if request.URL.Query().Get("group_by") != "" {
				if _, err := parseReportGroup(request.URL.Query().Get("group_by")); !errors.Is(
					err,
					ErrInvalidQuery,
				) {
					t.Fatal("invalid group accepted")
				}
			} else if _, err := ParseQuery(request); !errors.Is(
				err,
				ErrInvalidQuery,
			) {
				t.Fatal("invalid query accepted")
			}
		})
	}
	request := httptest.NewRequest(
		http.MethodGet,
		"/reports/grouped?geo_country=United+States&site_id=site%27quoted&empty=carrier&sort=epc&order=asc&limit=25&offset=50&min_clicks=10&profit=positive",
		nil,
	)
	query, err := ParseQuery(request)
	if err != nil {
		t.Fatal(err)
	}
	if query.Dimensions["geo_country"] != "United States" || query.Dimensions["site_id"] != "site'quoted" || query.Limit != 25 || query.Offset != 50 || query.MinClicks != 10 || query.Profit != "positive" || len(query.EmptyFields) != 1 {
		t.Fatalf(
			"incorrect validated query: %+v",
			query,
		)
	}
}

func TestCalendarReportDaysFollowTimezoneAndDST(t *testing.T) {
	for _, test := range []struct {
		date, zone string
		hours      int
		from       string
	}{
		{"2026-03-08", "America/New_York", 23, "2026-03-08T05:00:00Z"},
		{"2026-11-01", "America/New_York", 25, "2026-11-01T04:00:00Z"},
		{"2026-01-01", "Asia/Kolkata", 24, "2025-12-31T18:30:00Z"},
	} {
		request := httptest.NewRequest(
			http.MethodGet,
			"/reports/grouped?from="+test.date+"&to="+test.date+"&timezone="+test.zone,
			nil,
		)
		query, err := ParseQuery(request)
		if err != nil {
			t.Fatal(err)
		}
		if query.From.UTC().Format(time.RFC3339) != test.from || query.To.Sub(query.From)+time.Second != time.Duration(test.hours)*time.Hour {
			t.Fatalf(
				"incorrect timezone/day boundary: %+v",
				query,
			)
		}
	}
}
