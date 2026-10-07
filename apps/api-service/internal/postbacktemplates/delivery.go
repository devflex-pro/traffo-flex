package postbacktemplates

import (
	"context"
	"net/http"
	"time"
)

type DeliveryJob struct {
	ID           string    `json:"id"`
	TemplateID   string    `json:"template_id"`
	ConversionID string    `json:"conversion_id"`
	ClickID      string    `json:"click_id"`
	Status       string    `json:"status"`
	Attempts     int       `json:"attempts"`
	HTTPStatus   int       `json:"http_status"`
	Error        string    `json:"error"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type deliveryReader interface {
	DeliveryJobs(context.Context) (
		[]DeliveryJob,
		error,
	)
}

func (h *Handler) DeliveryJobs(
	w http.ResponseWriter,
	r *http.Request,
) {
	items := []DeliveryJob{}
	if repo, ok := h.service.repo.(deliveryReader); ok {
		var err error
		items, err = repo.DeliveryJobs(r.Context())
		if err != nil {
			h.respondError(
				w,
				http.StatusInternalServerError,
				"failed to list postback deliveries",
				err,
			)
			return
		}
	}
	h.respondJSON(
		w,
		http.StatusOK,
		map[string]any{"items": items},
	)
}
