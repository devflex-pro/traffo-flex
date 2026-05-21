package healthhistory

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/devflex/traffoflex/packages/go-shared/httpx"
)

var ErrInvalidQuery = errors.New("invalid health history query")

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
	DestinationID string
	Current       string
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
	DestinationID string `json:"destination_id"`
	Previous      string `json:"previous"`
	Current       string `json:"current"`
	Error         string `json:"error,omitempty"`
	CheckedAt     string `json:"checked_at"`
	UpdatedAt     string `json:"updated_at"`
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
			"failed to list health history",
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
			"failed to write health history response",
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
			"failed to write health history error",
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
	query := Query{Limit: 100}
	var err error
	query.DestinationID, err = parseOptionalFilter(
		"destination_id",
		values.Get("destination_id"),
	)
	if err != nil {
		return Query{}, err
	}
	query.Current, err = parseOptionalFilter(
		"current",
		values.Get("current"),
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
	if q.DestinationID != "" {
		filters["destination_id"] = q.DestinationID
	}
	if q.Current != "" {
		filters["current"] = q.Current
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
