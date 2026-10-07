package outbound

import (
	"errors"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"github.com/devflex/traffoflex/packages/go-shared/postback"
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
	return postback.Render(
		template.URL,
		Values(
			conversion,
			Click{},
		),
	)
}
