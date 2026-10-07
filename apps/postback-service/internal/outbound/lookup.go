package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type Click struct {
	OwnerID       string `json:"owner_id"`
	CampaignID    string `json:"campaign_id"`
	SourceID      string `json:"source_id"`
	SourceClickID string `json:"source_click_id"`
	Sub1          string `json:"sub1"`
	Sub2          string `json:"sub2"`
	Sub3          string `json:"sub3"`
	Sub4          string `json:"sub4"`
	Sub5          string `json:"sub5"`
	Sub6          string `json:"sub6"`
	Sub7          string `json:"sub7"`
	Sub8          string `json:"sub8"`
	Sub9          string `json:"sub9"`
	Sub10         string `json:"sub10"`
}

func (w *PersistentWorker) lookup(
	ctx context.Context,
	event models.ConversionEvent,
) (
	Click,
	bool,
	error,
) {
	quote := func(value string) string {
		value = strings.ReplaceAll(
			value,
			"\\",
			"\\\\",
		)
		return "'" + strings.ReplaceAll(
			value,
			"'",
			"\\'",
		) + "'"
	}
	query := "SELECT owner_id, campaign_id, source_id, source_click_id, sub1, sub2, sub3, sub4, sub5, sub6, sub7, sub8, sub9, sub10 FROM click_events WHERE click_id = " + quote(event.ClickID) + " AND owner_id = " + quote(event.OwnerID) + " ORDER BY created_at LIMIT 1 FORMAT JSONEachRow"
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		w.clickhouseURL,
		strings.NewReader(query),
	)
	if err != nil {
		return Click{}, false, errors.New("invalid click lookup request")
	}
	response, err := w.lookupClient.Do(request)
	if err != nil {
		return Click{}, false, errors.New("outbound click lookup unavailable")
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			w.log.Warn(
				"outbound click lookup close failed",
				"error",
				err,
			)
		}
	}()
	if response.StatusCode != http.StatusOK {
		return Click{}, false, errors.New("outbound click lookup rejected")
	}
	var click Click
	if err := json.NewDecoder(io.LimitReader(
		response.Body,
		65536,
	)).Decode(&click); err != nil {
		if errors.Is(
			err,
			io.EOF,
		) {
			return Click{}, false, nil
		}
		return Click{}, false, err
	}
	return click, true, nil
}
