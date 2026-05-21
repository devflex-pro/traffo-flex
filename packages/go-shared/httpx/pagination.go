package httpx

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
)

const (
	DefaultListLimit = 100
	MaxListLimit     = 500
)

type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type ListResponse[T any] struct {
	Items  []T `json:"items"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

func ParsePagination(r *http.Request) (
	Pagination,
	error,
) {
	values := r.URL.Query()
	pagination := Pagination{
		Limit: DefaultListLimit,
	}

	rawLimit := strings.TrimSpace(values.Get("limit"))
	if rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit <= 0 || limit > MaxListLimit {
			return Pagination{}, errors.New("limit is invalid")
		}
		pagination.Limit = limit
	}

	rawOffset := strings.TrimSpace(values.Get("offset"))
	if rawOffset != "" {
		offset, err := strconv.Atoi(rawOffset)
		if err != nil || offset < 0 {
			return Pagination{}, errors.New("offset is invalid")
		}
		pagination.Offset = offset
	}

	return pagination, nil
}

func NewListResponse[T any](
	items []T,
	pagination Pagination,
	total int,
) ListResponse[T] {
	if items == nil {
		items = []T{}
	}
	return ListResponse[T]{
		Items:  items,
		Limit:  pagination.Limit,
		Offset: pagination.Offset,
		Total:  total,
	}
}

func PaginateSlice[T any](
	items []T,
	pagination Pagination,
) (
	[]T,
	int,
) {
	total := len(items)
	if pagination.Offset >= total {
		return []T{}, total
	}
	end := pagination.Offset + pagination.Limit
	if end > total {
		end = total
	}
	return items[pagination.Offset:end], total
}
