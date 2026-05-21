package reports

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubRepository struct {
	overviewQuery Query
	groupBy       GroupBy
	groupedQuery  Query
	errorsQuery   IngestionErrorsQuery
	err           error
}

func (r *stubRepository) Overview(
	ctx context.Context,
	query Query,
) (
	Metrics,
	error,
) {
	if err := ctx.Err(); err != nil {
		return Metrics{}, err
	}
	r.overviewQuery = query
	if r.err != nil {
		return Metrics{}, r.err
	}
	return Metrics{
		Clicks:      10,
		Conversions: 2,
		Revenue:     12,
		Cost:        3,
		Profit:      9,
		ROI:         300,
	}, nil
}

func (r *stubRepository) IngestionErrors(
	ctx context.Context,
	query IngestionErrorsQuery,
) (
	IngestionErrorsReport,
	error,
) {
	if err := ctx.Err(); err != nil {
		return IngestionErrorsReport{}, err
	}
	r.errorsQuery = query
	if r.err != nil {
		return IngestionErrorsReport{}, r.err
	}
	return IngestionErrorsReport{
		Rows: []IngestionErrorRow{
			{
				Topic:      "traffoflex.click_events",
				Error:      "invalid json",
				RawMessage: "{",
			},
		},
		Filters: query.Filters(),
	}, nil
}

func (r *stubRepository) Grouped(
	ctx context.Context,
	groupBy GroupBy,
	query Query,
) (
	GroupedReport,
	error,
) {
	if err := ctx.Err(); err != nil {
		return GroupedReport{}, err
	}
	r.groupBy = groupBy
	r.groupedQuery = query
	if r.err != nil {
		return GroupedReport{}, r.err
	}
	return GroupedReport{
		GroupBy: string(groupBy),
		Summary: Metrics{Clicks: 10},
		Rows: []ReportRow{
			{
				ID:      "cmp_1",
				Name:    "cmp_1",
				Metrics: Metrics{Clicks: 10},
			},
		},
		Filters: query.Filters(),
	}, nil
}

func (r *stubRepository) Daily(
	ctx context.Context,
	query Query,
) (
	GroupedReport,
	error,
) {
	if err := ctx.Err(); err != nil {
		return GroupedReport{}, err
	}
	r.groupedQuery = query
	if r.err != nil {
		return GroupedReport{}, r.err
	}
	return GroupedReport{
		GroupBy: "day",
		Summary: Metrics{Clicks: 10},
		Rows: []ReportRow{
			{
				ID:      "2026-01-01",
				Name:    "2026-01-01",
				Metrics: Metrics{Clicks: 10},
			},
		},
		Filters: query.Filters(),
	}, nil
}

func TestOverviewUsesRepositoryWithValidatedQuery(t *testing.T) {
	repo := &stubRepository{}
	handler := NewHandlerWithRepository(
		testLogger(),
		repo,
	)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reports/overview?from=2026-01-01&to=2026-01-31&timezone=Europe/Moscow&campaign_id=cmp_1",
		nil,
	)
	rr := httptest.NewRecorder()

	handler.Overview(
		rr,
		req,
	)

	if rr.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body=%s",
			rr.Code,
			http.StatusOK,
			rr.Body.String(),
		)
	}
	if repo.overviewQuery.CampaignID != "cmp_1" {
		t.Fatalf(
			"campaign filter = %q, want cmp_1",
			repo.overviewQuery.CampaignID,
		)
	}
	if repo.overviewQuery.Timezone != "Europe/Moscow" {
		t.Fatalf(
			"timezone = %q, want Europe/Moscow",
			repo.overviewQuery.Timezone,
		)
	}

	var body Metrics
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf(
			"decode response: %v",
			err,
		)
	}
	if body.Clicks != 10 {
		t.Fatalf(
			"clicks = %d, want 10",
			body.Clicks,
		)
	}
}

func TestGroupedUsesRepository(t *testing.T) {
	repo := &stubRepository{}
	handler := NewHandlerWithRepository(
		testLogger(),
		repo,
	)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reports/campaigns?source_id=src_1",
		nil,
	)
	rr := httptest.NewRecorder()

	handler.Campaigns(
		rr,
		req,
	)

	if rr.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body=%s",
			rr.Code,
			http.StatusOK,
			rr.Body.String(),
		)
	}
	if repo.groupBy != GroupByCampaign {
		t.Fatalf(
			"group by = %q, want %q",
			repo.groupBy,
			GroupByCampaign,
		)
	}
	if repo.groupedQuery.SourceID != "src_1" {
		t.Fatalf(
			"source filter = %q, want src_1",
			repo.groupedQuery.SourceID,
		)
	}
}

func TestInvalidQueryReturnsBadRequest(t *testing.T) {
	handler := NewHandlerWithRepository(
		testLogger(),
		&stubRepository{},
	)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reports/overview?from=2026-02-01&to=2026-01-01",
		nil,
	)
	rr := httptest.NewRecorder()

	handler.Overview(
		rr,
		req,
	)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d",
			rr.Code,
			http.StatusBadRequest,
		)
	}
}

func TestIngestionErrorsUsesRepository(t *testing.T) {
	repo := &stubRepository{}
	handler := NewHandlerWithRepository(
		testLogger(),
		repo,
	)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reports/ingestion-errors?topic=traffoflex.click_events&limit=25",
		nil,
	)
	rr := httptest.NewRecorder()

	handler.IngestionErrors(
		rr,
		req,
	)

	if rr.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body=%s",
			rr.Code,
			http.StatusOK,
			rr.Body.String(),
		)
	}
	if repo.errorsQuery.Topic != "traffoflex.click_events" {
		t.Fatalf(
			"topic = %q, want traffoflex.click_events",
			repo.errorsQuery.Topic,
		)
	}
	if repo.errorsQuery.Limit != 25 {
		t.Fatalf(
			"limit = %d, want 25",
			repo.errorsQuery.Limit,
		)
	}
}

func TestRepositoryErrorReturnsServerError(t *testing.T) {
	handler := NewHandlerWithRepository(
		testLogger(),
		&stubRepository{err: errors.New("repository unavailable")},
	)
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/reports/overview",
		nil,
	)
	rr := httptest.NewRecorder()

	handler.Overview(
		rr,
		req,
	)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want %d",
			rr.Code,
			http.StatusInternalServerError,
		)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(
		io.Discard,
		nil,
	))
}
