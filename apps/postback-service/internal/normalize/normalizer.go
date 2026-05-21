package normalize

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

var (
	ErrInvalidPayload = errors.New("invalid postback payload")
	ErrUnauthorized   = errors.New("invalid postback secret")
)

type Template struct {
	NetworkID string
	Secret    string
}

type Conversion struct {
	NetworkID     string            `json:"network_id"`
	ClickID       string            `json:"click_id"`
	TransactionID string            `json:"transaction_id"`
	EventType     string            `json:"event_type"`
	Status        string            `json:"status"`
	Payout        float64           `json:"payout"`
	Currency      string            `json:"currency"`
	RawPayload    map[string]string `json:"raw_payload"`
}

func FromGET(
	r *http.Request,
	template Template,
) (
	Conversion,
	error,
) {
	payload := valuesToMap(r.URL.Query())
	return normalizePayload(
		payload,
		template,
	)
}

func FromPOST(
	r *http.Request,
	template Template,
) (
	Conversion,
	error,
) {
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(
		contentType,
		"application/json",
	) {
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			return Conversion{}, errors.Join(
				ErrInvalidPayload,
				err,
			)
		}
		return normalizePayload(
			payload,
			template,
		)
	}
	if err := r.ParseForm(); err != nil {
		return Conversion{}, errors.Join(
			ErrInvalidPayload,
			err,
		)
	}
	return normalizePayload(
		valuesToMap(r.Form),
		template,
	)
}

func normalizePayload(
	payload map[string]string,
	template Template,
) (
	Conversion,
	error,
) {
	if err := validateSecret(
		payload,
		template.Secret,
	); err != nil {
		return Conversion{}, err
	}

	clickID := first(
		payload["click_id"],
		payload["cid"],
		payload["subid"],
		payload["sub_id"],
	)
	transactionID := first(
		payload["transaction_id"],
		payload["tx"],
		payload["tid"],
		payload["order_id"],
	)
	if clickID == "" {
		return Conversion{}, errors.Join(
			ErrInvalidPayload,
			errors.New("click id is required"),
		)
	}
	if transactionID == "" {
		return Conversion{}, errors.Join(
			ErrInvalidPayload,
			errors.New("transaction id is required"),
		)
	}

	return Conversion{
		NetworkID:     template.NetworkID,
		ClickID:       clickID,
		TransactionID: transactionID,
		EventType: first(
			payload["event_type"],
			payload["event"],
			"conversion",
		),
		Status: first(
			payload["status"],
			"approved",
		),
		Payout: parseFloat(first(
			payload["payout"],
			payload["sum"],
			payload["amount"],
			payload["revenue"],
		)),
		Currency: first(
			payload["currency"],
			"USD",
		),
		RawPayload: payload,
	}, nil
}

func validateSecret(
	payload map[string]string,
	secret string,
) error {
	if secret == "" {
		return nil
	}
	got := first(
		payload["secret"],
		payload["token"],
		payload["key"],
	)
	if got != secret {
		return ErrUnauthorized
	}
	return nil
}

func valuesToMap(values map[string][]string) map[string]string {
	payload := make(
		map[string]string,
		len(values),
	)
	for key, items := range values {
		if len(items) > 0 {
			payload[key] = items[0]
		}
	}
	return payload
}

func parseFloat(value string) float64 {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(
		value,
		64,
	)
	if err != nil {
		return 0
	}
	return parsed
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
