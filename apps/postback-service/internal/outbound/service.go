package outbound

import (
	"context"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/ids"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

type Service struct {
	queue     Queue
	templates []Template
}

func NewService(
	queue Queue,
	templates []Template,
) *Service {
	if queue == nil {
		queue = NewMemoryQueue(100)
	}
	return &Service{queue: queue, templates: templates}
}

func (s *Service) EnqueueForConversion(
	ctx context.Context,
	conversion models.ConversionEvent,
) error {
	for _, template := range s.templates {
		targetURL, err := Render(
			template,
			conversion,
		)
		if err != nil {
			continue
		}
		if err := s.queue.Enqueue(ctx, Job{
			ID:          ids.New("out"),
			TemplateID:  template.ID,
			URL:         targetURL,
			MaxAttempts: 3,
			CreatedAt:   time.Now().UTC(),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Retry(ctx context.Context) (
	int,
	error,
) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return 0, nil
}
