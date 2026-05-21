package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestParsePaginationDefaults(t *testing.T) {
	req := httptest.NewRequest(
		"GET",
		"/items",
		nil,
	)
	pagination, err := ParsePagination(req)
	if err != nil {
		t.Fatalf(
			"ParsePagination returned error: %v",
			err,
		)
	}
	if pagination.Limit != DefaultListLimit {
		t.Fatalf(
			"limit = %d, want %d",
			pagination.Limit,
			DefaultListLimit,
		)
	}
	if pagination.Offset != 0 {
		t.Fatalf(
			"offset = %d, want 0",
			pagination.Offset,
		)
	}
}

func TestParsePaginationRejectsInvalidValues(t *testing.T) {
	cases := []string{
		"/items?limit=0",
		"/items?limit=501",
		"/items?limit=abc",
		"/items?offset=-1",
		"/items?offset=abc",
	}
	for _, path := range cases {
		req := httptest.NewRequest(
			"GET",
			path,
			nil,
		)
		if _, err := ParsePagination(req); err == nil {
			t.Fatalf(
				"ParsePagination(%q) expected error",
				path,
			)
		}
	}
}

func TestPaginateSlice(t *testing.T) {
	items, total := PaginateSlice(
		[]int{1, 2, 3, 4},
		Pagination{
			Limit:  2,
			Offset: 1,
		},
	)
	if total != 4 {
		t.Fatalf(
			"total = %d, want 4",
			total,
		)
	}
	if len(items) != 2 || items[0] != 2 || items[1] != 3 {
		t.Fatalf(
			"items = %#v, want []int{2, 3}",
			items,
		)
	}
}
