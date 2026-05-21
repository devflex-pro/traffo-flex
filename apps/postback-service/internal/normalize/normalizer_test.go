package normalize

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFromGETNormalizesAliases(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/pb/demo?cid=clk_1&tx=tx_1&sum=12.5&currency=EUR&status=approved&secret=s",
		nil,
	)

	conversion, err := FromGET(
		req,
		Template{NetworkID: "demo", Secret: "s"},
	)
	if err != nil {
		t.Fatalf(
			"FromGET returned error: %v",
			err,
		)
	}
	if conversion.ClickID != "clk_1" {
		t.Fatalf(
			"ClickID = %q, want clk_1",
			conversion.ClickID,
		)
	}
	if conversion.TransactionID != "tx_1" {
		t.Fatalf(
			"TransactionID = %q, want tx_1",
			conversion.TransactionID,
		)
	}
	if conversion.Payout != 12.5 {
		t.Fatalf(
			"Payout = %v, want 12.5",
			conversion.Payout,
		)
	}
	if conversion.Currency != "EUR" {
		t.Fatalf(
			"Currency = %q, want EUR",
			conversion.Currency,
		)
	}
}

func TestFromPOSTJSON(t *testing.T) {
	body := strings.NewReader(`{"click_id":"clk_1","transaction_id":"tx_1","payout":"3.14"}`)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/postbacks",
		body,
	)
	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	conversion, err := FromPOST(
		req,
		Template{NetworkID: "api"},
	)
	if err != nil {
		t.Fatalf(
			"FromPOST returned error: %v",
			err,
		)
	}
	if conversion.EventType != "conversion" {
		t.Fatalf(
			"EventType = %q, want conversion",
			conversion.EventType,
		)
	}
	if conversion.Payout != 3.14 {
		t.Fatalf(
			"Payout = %v, want 3.14",
			conversion.Payout,
		)
	}
}

func TestFromPOSTForm(t *testing.T) {
	body := strings.NewReader("subid=clk_1&order_id=tx_1")
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/postbacks",
		body,
	)
	req.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	conversion, err := FromPOST(
		req,
		Template{NetworkID: "api"},
	)
	if err != nil {
		t.Fatalf(
			"FromPOST returned error: %v",
			err,
		)
	}
	if conversion.ClickID != "clk_1" {
		t.Fatalf(
			"ClickID = %q, want clk_1",
			conversion.ClickID,
		)
	}
}

func TestSecretValidation(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/pb/demo?cid=clk_1&tx=tx_1&secret=bad",
		nil,
	)

	if _, err := FromGET(
		req,
		Template{NetworkID: "demo", Secret: "good"},
	); err == nil {
		t.Fatal("expected secret validation error")
	}
}

func TestMissingRequiredFields(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodGet,
		"/pb/demo?cid=clk_1",
		nil,
	)

	if _, err := FromGET(
		req,
		Template{NetworkID: "demo"},
	); err == nil {
		t.Fatal("expected missing transaction id error")
	}
}
