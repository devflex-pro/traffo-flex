package reports

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/httpx"
)

var (
	ErrInvalidQuery = errors.New("invalid report query")
	idPattern       = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	topicPattern    = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,256}$`)
)

type Handler struct {
	log  *slog.Logger
	repo Repository
}

func NewHandler(log *slog.Logger) *Handler {
	return NewHandlerWithRepository(
		log,
		MemoryRepository{},
	)
}

func NewHandlerWithRepository(
	log *slog.Logger,
	repo Repository,
) *Handler {
	if repo == nil {
		repo = MemoryRepository{}
	}
	return &Handler{
		log:  log,
		repo: repo,
	}
}

type Repository interface {
	Overview(
		ctx context.Context,
		query Query,
	) (
		Metrics,
		error,
	)
	Grouped(
		ctx context.Context,
		groupBy GroupBy,
		query Query,
	) (
		GroupedReport,
		error,
	)
	Daily(
		ctx context.Context,
		query Query,
	) (
		GroupedReport,
		error,
	)
	IngestionErrors(
		ctx context.Context,
		query IngestionErrorsQuery,
	) (
		IngestionErrorsReport,
		error,
	)
}

type MemoryRepository struct{}

func (MemoryRepository) Overview(
	ctx context.Context,
	query Query,
) (
	Metrics,
	error,
) {
	if err := ctx.Err(); err != nil {
		return Metrics{}, err
	}
	return Metrics{}, nil
}

func (MemoryRepository) Grouped(
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
	return GroupedReport{
		GroupBy: string(groupBy),
		Summary: Metrics{},
		Rows:    []ReportRow{},
		Filters: query.Filters(),
	}, nil
}

func (MemoryRepository) Daily(
	ctx context.Context,
	query Query,
) (
	GroupedReport,
	error,
) {
	if err := ctx.Err(); err != nil {
		return GroupedReport{}, err
	}
	return GroupedReport{
		GroupBy: "day",
		Summary: Metrics{},
		Rows:    []ReportRow{},
		Filters: query.Filters(),
	}, nil
}

func (MemoryRepository) IngestionErrors(
	ctx context.Context,
	query IngestionErrorsQuery,
) (
	IngestionErrorsReport,
	error,
) {
	if err := ctx.Err(); err != nil {
		return IngestionErrorsReport{}, err
	}
	return IngestionErrorsReport{
		Rows:    []IngestionErrorRow{},
		Filters: query.Filters(),
	}, nil
}

type GroupBy string

const (
	GroupByCampaign    GroupBy = "campaign"
	GroupByStream      GroupBy = "stream"
	GroupByDestination GroupBy = "destination"
	GroupBySource      GroupBy = "source"
	GroupByTrafficback GroupBy = "trafficback"
	GroupByHealth      GroupBy = "health"
)

type Query struct {
	OwnerID       string
	From          time.Time
	To            time.Time
	Timezone      string
	CampaignID    string
	StreamID      string
	DestinationID string
	SourceID      string
}

type IngestionErrorsQuery struct {
	From  time.Time
	To    time.Time
	Topic string
	Limit int
}

type IngestionErrorsReport struct {
	Rows    []IngestionErrorRow `json:"rows"`
	Filters map[string]string   `json:"filters"`
}

type IngestionErrorRow struct {
	ObservedAt string `json:"observed_at"`
	Topic      string `json:"topic"`
	Error      string `json:"error"`
	RawMessage string `json:"raw_message"`
}

func ParseIngestionErrorsQuery(r *http.Request) (
	IngestionErrorsQuery,
	error,
) {
	values := r.URL.Query()
	from, err := parseOptionalTime(values.Get("from"))
	if err != nil {
		return IngestionErrorsQuery{}, errors.Join(
			ErrInvalidQuery,
			errors.New("invalid from"),
		)
	}
	to, err := parseOptionalTime(values.Get("to"))
	if err != nil {
		return IngestionErrorsQuery{}, errors.Join(
			ErrInvalidQuery,
			errors.New("invalid to"),
		)
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return IngestionErrorsQuery{}, errors.Join(
			ErrInvalidQuery,
			errors.New("from must be before to"),
		)
	}
	limit := 100
	rawLimit := strings.TrimSpace(values.Get("limit"))
	if rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 || parsed > 500 {
			return IngestionErrorsQuery{}, errors.Join(
				ErrInvalidQuery,
				errors.New("limit is invalid"),
			)
		}
		limit = parsed
	}
	topic := strings.TrimSpace(values.Get("topic"))
	if topic != "" && !topicPattern.MatchString(topic) {
		return IngestionErrorsQuery{}, errors.Join(
			ErrInvalidQuery,
			errors.New("topic is invalid"),
		)
	}
	return IngestionErrorsQuery{
		From:  from,
		To:    to,
		Topic: topic,
		Limit: limit,
	}, nil
}

func ParseQuery(r *http.Request) (
	Query,
	error,
) {
	values := r.URL.Query()
	query := Query{
		Timezone: strings.TrimSpace(values.Get("timezone")),
	}
	if query.Timezone == "" {
		query.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(query.Timezone); err != nil {
		return Query{}, errors.Join(
			ErrInvalidQuery,
			errors.New("invalid timezone"),
		)
	}

	from, err := parseOptionalTime(values.Get("from"))
	if err != nil {
		return Query{}, errors.Join(
			ErrInvalidQuery,
			errors.New("invalid from"),
		)
	}
	to, err := parseOptionalTime(values.Get("to"))
	if err != nil {
		return Query{}, errors.Join(
			ErrInvalidQuery,
			errors.New("invalid to"),
		)
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return Query{}, errors.Join(
			ErrInvalidQuery,
			errors.New("from must be before to"),
		)
	}
	query.From = from
	query.To = to

	var filterErr error
	query.CampaignID, filterErr = parseOptionalID(
		"campaign_id",
		values.Get("campaign_id"),
	)
	if filterErr != nil {
		return Query{}, filterErr
	}
	query.StreamID, filterErr = parseOptionalID(
		"stream_id",
		values.Get("stream_id"),
	)
	if filterErr != nil {
		return Query{}, filterErr
	}
	query.DestinationID, filterErr = parseOptionalID(
		"destination_id",
		values.Get("destination_id"),
	)
	if filterErr != nil {
		return Query{}, filterErr
	}
	query.SourceID, filterErr = parseOptionalID(
		"source_id",
		values.Get("source_id"),
	)
	if filterErr != nil {
		return Query{}, filterErr
	}

	return query, nil
}

func parseOptionalTime(raw string) (
	time.Time,
	error,
) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.Parse(
		time.DateOnly,
		raw,
	); err == nil {
		return parsed, nil
	}
	return time.Parse(
		time.RFC3339,
		raw,
	)
}

func parseOptionalID(
	name,
	raw string,
) (
	string,
	error,
) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if !idPattern.MatchString(raw) {
		return "", errors.Join(
			ErrInvalidQuery,
			errors.New(name+" is invalid"),
		)
	}
	return raw, nil
}

func (q Query) Filters() map[string]string {
	filters := make(map[string]string)
	if !q.From.IsZero() {
		filters["from"] = q.From.Format(time.DateOnly)
	}
	if !q.To.IsZero() {
		filters["to"] = q.To.Format(time.DateOnly)
	}
	if q.Timezone != "" {
		filters["timezone"] = q.Timezone
	}
	if q.CampaignID != "" {
		filters["campaign_id"] = q.CampaignID
	}
	if q.StreamID != "" {
		filters["stream_id"] = q.StreamID
	}
	if q.DestinationID != "" {
		filters["destination_id"] = q.DestinationID
	}
	if q.SourceID != "" {
		filters["source_id"] = q.SourceID
	}
	return filters
}

func (q IngestionErrorsQuery) Filters() map[string]string {
	filters := make(map[string]string)
	if !q.From.IsZero() {
		filters["from"] = q.From.Format(time.DateOnly)
	}
	if !q.To.IsZero() {
		filters["to"] = q.To.Format(time.DateOnly)
	}
	if q.Topic != "" {
		filters["topic"] = q.Topic
	}
	if q.Limit > 0 {
		filters["limit"] = strconv.Itoa(q.Limit)
	}
	return filters
}

type Metrics struct {
	Clicks      int     `json:"clicks"`
	Conversions int     `json:"conversions"`
	Revenue     float64 `json:"revenue"`
	Cost        float64 `json:"cost"`
	Profit      float64 `json:"profit"`
	ROI         float64 `json:"roi"`
	Events      int     `json:"events,omitempty"`
}

type GroupedReport struct {
	GroupBy string            `json:"group_by"`
	Summary Metrics           `json:"summary"`
	Rows    []ReportRow       `json:"rows"`
	Filters map[string]string `json:"filters"`
}

type ReportRow struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Metrics Metrics `json:"metrics"`
}

func (h *Handler) Overview(
	w http.ResponseWriter,
	r *http.Request,
) {
	query, err := ParseQuery(r)
	if err != nil {
		h.respondError(
			w,
			http.StatusBadRequest,
			err.Error(),
			err,
		)
		return
	}
	metrics, err := h.repo.Overview(
		r.Context(),
		query,
	)
	if err != nil {
		h.respondError(
			w,
			http.StatusInternalServerError,
			"failed to load overview report",
			err,
		)
		return
	}
	h.respondJSON(
		w,
		"overview",
		metrics,
	)
}

func (h *Handler) Daily(
	w http.ResponseWriter,
	r *http.Request,
) {
	query, err := ParseQuery(r)
	if err != nil {
		h.respondError(
			w,
			http.StatusBadRequest,
			err.Error(),
			err,
		)
		return
	}
	report, err := h.repo.Daily(
		r.Context(),
		query,
	)
	if err != nil {
		h.respondError(
			w,
			http.StatusInternalServerError,
			"failed to load daily report",
			err,
		)
		return
	}
	h.respondJSON(
		w,
		"reports daily response",
		report,
	)
}

func (h *Handler) Campaigns(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.respondGrouped(
		w,
		r,
		GroupByCampaign,
	)
}

func (h *Handler) Streams(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.respondGrouped(
		w,
		r,
		GroupByStream,
	)
}

func (h *Handler) Destinations(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.respondGrouped(
		w,
		r,
		GroupByDestination,
	)
}

func (h *Handler) Sources(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.respondGrouped(
		w,
		r,
		GroupBySource,
	)
}

func (h *Handler) Trafficback(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.respondGrouped(
		w,
		r,
		GroupByTrafficback,
	)
}

func (h *Handler) Health(
	w http.ResponseWriter,
	r *http.Request,
) {
	h.respondGrouped(
		w,
		r,
		GroupByHealth,
	)
}

func (h *Handler) IngestionErrors(
	w http.ResponseWriter,
	r *http.Request,
) {
	query, err := ParseIngestionErrorsQuery(r)
	if err != nil {
		h.respondError(
			w,
			http.StatusBadRequest,
			err.Error(),
			err,
		)
		return
	}
	report, err := h.repo.IngestionErrors(
		r.Context(),
		query,
	)
	if err != nil {
		h.respondError(
			w,
			http.StatusInternalServerError,
			"failed to load ingestion errors",
			err,
		)
		return
	}
	h.respondJSON(
		w,
		"ingestion errors response",
		report,
	)
}

func (h *Handler) respondGrouped(
	w http.ResponseWriter,
	r *http.Request,
	groupBy GroupBy,
) {
	query, err := ParseQuery(r)
	if err != nil {
		h.respondError(
			w,
			http.StatusBadRequest,
			err.Error(),
			err,
		)
		return
	}
	report, err := h.repo.Grouped(
		r.Context(),
		groupBy,
		query,
	)
	if err != nil {
		h.respondError(
			w,
			http.StatusInternalServerError,
			"failed to load "+string(groupBy)+" report",
			err,
		)
		return
	}
	h.respondJSON(
		w,
		"reports "+string(groupBy)+" response",
		report,
	)
}

func (h *Handler) respondJSON(
	w http.ResponseWriter,
	operation string,
	data any,
) {
	if err := httpx.JSON(
		w,
		http.StatusOK,
		data,
	); err != nil {
		h.log.Error(
			"failed to write "+operation,
			"error",
			err,
		)
	}
}

func (h *Handler) respondError(
	w http.ResponseWriter,
	status int,
	message string,
	err error,
) {
	if status >= http.StatusInternalServerError {
		h.log.Error(
			message,
			"error",
			err,
		)
	}
	if writeErr := httpx.Error(
		w,
		status,
		message,
	); writeErr != nil {
		h.log.Error(
			"failed to write reports error",
			"error",
			writeErr,
		)
	}
}
