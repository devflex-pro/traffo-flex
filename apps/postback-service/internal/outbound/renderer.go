package outbound

import (
	"errors"
	"net/url"

	"github.com/devflex/traffoflex/packages/go-shared/macros"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

var ErrInvalidTemplate = errors.New("invalid outbound postback template")

type Template struct {
	ID      string
	URL     string
	Enabled bool
}

func Render(
	template Template,
	conversion models.ConversionEvent,
) (
	string,
	error,
) {
	if !template.Enabled {
		return "", ErrInvalidTemplate
	}
	if _, err := url.ParseRequestURI(template.URL); err != nil {
		return "", errors.Join(
			ErrInvalidTemplate,
			err,
		)
	}

	return macros.Render(template.URL, map[string]string{
		"conversion_id":  conversion.ConversionID,
		"click_id":       conversion.ClickID,
		"transaction_id": conversion.TransactionID,
		"event_type":     conversion.EventType,
		"status":         conversion.Status,
		"currency":       conversion.Currency,
		"network_id":     conversion.NetworkID,
	}), nil
}
