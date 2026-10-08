package postbacklogs

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

var ErrInvalidQuery = errors.New("invalid postback log query")

var filterPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,256}$`)

type Repository interface {
	List(
		ctx context.Context,
		query Query,
	) (
		Report,
		error,
	)
}

type Query struct {
	ID            string
	From          time.Time
	To            time.Time
	Order         string
	NetworkID     string
	ClickID       string
	TransactionID string
	Status        string
	Limit         int
	Offset        int
}

type Report struct {
	Items   []Row             `json:"items"`
	Filters map[string]string `json:"filters"`
	Limit   int               `json:"limit"`
	Offset  int               `json:"offset"`
	Total   int               `json:"total"`
}

type Row struct {
	CreatedAt     string            `json:"created_at"`
	PostbackID    string            `json:"postback_id"`
	NetworkID     string            `json:"network_id"`
	ClickID       string            `json:"click_id,omitempty"`
	TransactionID string            `json:"transaction_id,omitempty"`
	Payout        *float64          `json:"payout"`
	Currency      string            `json:"currency"`
	Status        string            `json:"status"`
	Error         string            `json:"error,omitempty"`
	RawPayload    map[string]string `json:"raw_payload"`
}

type Handler struct {
	log  *slog.Logger
	repo Repository
}

func NewHandler(
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

func (h *Handler) List(
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
	report, err := h.repo.List(
		r.Context(),
		query,
	)
	if err != nil {
		h.respondError(
			w,
			http.StatusInternalServerError,
			"failed to list postback logs",
			err,
		)
		return
	}
	if err := httpx.JSON(
		w,
		http.StatusOK,
		report,
	); err != nil {
		h.log.Error(
			"failed to write postback logs response",
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
			"failed to write postback logs error",
			"error",
			writeErr,
		)
	}
}

func ParseQuery(r *http.Request) (
	Query,
	error,
) {
	values := r.URL.Query()
	query := Query{
		Limit: 100,
		Order: "desc",
	}
	var err error
	query.ID, err = parseOptionalFilter(
		"id",
		values.Get("id"),
	)
	if err != nil {
		return Query{}, err
	}
	for name, target := range map[string]*time.Time{"from": &query.From, "to": &query.To} {
		if raw := strings.TrimSpace(values.Get(name)); raw != "" {
			parsed, parseErr := time.Parse(
				time.RFC3339Nano,
				raw,
			)
			if parseErr != nil {
				return Query{}, errors.Join(
					ErrInvalidQuery,
					errors.New(name+" must be an RFC3339 timestamp"),
				)
			}
			*target = parsed.UTC()
		}
	}
	if !query.From.IsZero() && !query.To.IsZero() && query.From.After(query.To) {
		return Query{}, errors.Join(
			ErrInvalidQuery,
			errors.New("from must not be after to"),
		)
	}
	if order := strings.TrimSpace(values.Get("order")); order != "" {
		if order != "asc" && order != "desc" {
			return Query{}, errors.Join(
				ErrInvalidQuery,
				errors.New("order must be asc or desc"),
			)
		}
		query.Order = order
	}
	query.NetworkID, err = parseOptionalFilter(
		"network_id",
		values.Get("network_id"),
	)
	if err != nil {
		return Query{}, err
	}
	query.ClickID, err = parseOptionalFilter(
		"click_id",
		values.Get("click_id"),
	)
	if err != nil {
		return Query{}, err
	}
	query.TransactionID, err = parseOptionalFilter(
		"transaction_id",
		values.Get("transaction_id"),
	)
	if err != nil {
		return Query{}, err
	}
	query.Status, err = parseOptionalFilter(
		"status",
		values.Get("status"),
	)
	if err != nil {
		return Query{}, err
	}
	rawLimit := strings.TrimSpace(values.Get("limit"))
	if rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit <= 0 || limit > 500 {
			return Query{}, errors.Join(
				ErrInvalidQuery,
				errors.New("limit is invalid"),
			)
		}
		query.Limit = limit
	}
	rawOffset := strings.TrimSpace(values.Get("offset"))
	if rawOffset != "" {
		offset, err := strconv.Atoi(rawOffset)
		if err != nil || offset < 0 {
			return Query{}, errors.Join(
				ErrInvalidQuery,
				errors.New("offset is invalid"),
			)
		}
		query.Offset = offset
	}
	return query, nil
}

func parseOptionalFilter(
	name string,
	raw string,
) (
	string,
	error,
) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if !filterPattern.MatchString(raw) {
		return "", errors.Join(
			ErrInvalidQuery,
			errors.New(name+" is invalid"),
		)
	}
	return raw, nil
}

func (q Query) Filters() map[string]string {
	filters := make(map[string]string)
	if q.ID != "" {
		filters["id"] = q.ID
	}
	if !q.From.IsZero() {
		filters["from"] = q.From.Format(time.RFC3339Nano)
	}
	if !q.To.IsZero() {
		filters["to"] = q.To.Format(time.RFC3339Nano)
	}
	if q.Order != "" {
		filters["order"] = q.Order
	}
	if q.NetworkID != "" {
		filters["network_id"] = q.NetworkID
	}
	if q.ClickID != "" {
		filters["click_id"] = q.ClickID
	}
	if q.TransactionID != "" {
		filters["transaction_id"] = q.TransactionID
	}
	if q.Status != "" {
		filters["status"] = q.Status
	}
	if q.Limit > 0 {
		filters["limit"] = strconv.Itoa(q.Limit)
	}
	if q.Offset > 0 {
		filters["offset"] = strconv.Itoa(q.Offset)
	}
	return filters
}

type MemoryRepository struct{}

func (MemoryRepository) List(
	ctx context.Context,
	query Query,
) (
	Report,
	error,
) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	return Report{
		Items:   []Row{},
		Filters: query.Filters(),
		Limit:   query.Limit,
		Offset:  query.Offset,
		Total:   0,
	}, nil
}
