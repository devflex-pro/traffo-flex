package conversions

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestClickHouseClickLookupFindsClickInfo(t *testing.T) {
	doer := roundTripFunc(func(req *http.Request) (
		*http.Response,
		error,
	) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		if !strings.Contains(
			string(body),
			"WHERE click_id = 'clk_1'",
		) {
			t.Fatalf(
				"query does not filter click_id: %s",
				string(body),
			)
		}
		return textResponse(
			http.StatusOK,
			`{"campaign_id":"cmp_1","stream_id":"str_1","destination_id":"dst_1","source_id":"src_1"}`+"\n",
		), nil
	})
	lookup := NewClickHouseClickLookupWithDoer(
		"http://clickhouse:8123?database=traffoflex",
		time.Second,
		doer,
	)

	info, found, err := lookup.Find(
		context.Background(),
		"clk_1",
	)
	if err != nil {
		t.Fatalf(
			"Find returned error: %v",
			err,
		)
	}
	if !found {
		t.Fatal("found = false, want true")
	}
	if info.CampaignID != "cmp_1" {
		t.Fatalf(
			"CampaignID = %q, want cmp_1",
			info.CampaignID,
		)
	}
	if info.StreamID != "str_1" {
		t.Fatalf(
			"StreamID = %q, want str_1",
			info.StreamID,
		)
	}
	if info.DestinationID != "dst_1" {
		t.Fatalf(
			"DestinationID = %q, want dst_1",
			info.DestinationID,
		)
	}
	if info.SourceID != "src_1" {
		t.Fatalf(
			"SourceID = %q, want src_1",
			info.SourceID,
		)
	}
}

func TestClickHouseClickLookupReturnsNotFoundForEmptyResponse(t *testing.T) {
	lookup := NewClickHouseClickLookupWithDoer(
		"http://clickhouse:8123?database=traffoflex",
		time.Second,
		roundTripFunc(func(req *http.Request) (
			*http.Response,
			error,
		) {
			return textResponse(
				http.StatusOK,
				"",
			), nil
		}),
	)

	_, found, err := lookup.Find(
		context.Background(),
		"clk_missing",
	)
	if err != nil {
		t.Fatalf(
			"Find returned error: %v",
			err,
		)
	}
	if found {
		t.Fatal("found = true, want false")
	}
}

func TestEscapeClickHouseString(t *testing.T) {
	got := escapeClickHouseString("clk_'\\")
	if got != "clk_''\\\\" {
		t.Fatalf(
			"escaped value = %q",
			got,
		)
	}
}

type roundTripFunc func(req *http.Request) (
	*http.Response,
	error,
)

func (f roundTripFunc) Do(req *http.Request) (
	*http.Response,
	error,
) {
	return f(req)
}

func textResponse(
	status int,
	body string,
) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body: io.NopCloser(strings.NewReader(
			body,
		)),
	}
}
