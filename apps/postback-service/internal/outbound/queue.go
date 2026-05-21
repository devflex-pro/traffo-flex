package outbound

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrQueueFull = errors.New("outbound postback queue is full")

type Job struct {
	ID          string    `json:"id"`
	TemplateID  string    `json:"template_id"`
	URL         string    `json:"url"`
	Attempts    int       `json:"attempts"`
	MaxAttempts int       `json:"max_attempts"`
	CreatedAt   time.Time `json:"created_at"`
}

type Queue interface {
	Enqueue(
		ctx context.Context,
		job Job,
	) error
}

type MemoryQueue struct {
	mu   sync.RWMutex
	jobs []Job
	max  int
}

func NewMemoryQueue(max int) *MemoryQueue {
	if max <= 0 {
		max = 100
	}
	return &MemoryQueue{max: max}
}

func (q *MemoryQueue) Enqueue(
	ctx context.Context,
	job Job,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.jobs) >= q.max {
		return ErrQueueFull
	}
	q.jobs = append(
		q.jobs,
		job,
	)
	return nil
}

func (q *MemoryQueue) Jobs() []Job {
	q.mu.RLock()
	defer q.mu.RUnlock()
	jobs := make(
		[]Job,
		len(q.jobs),
	)
	copy(
		jobs,
		q.jobs,
	)
	return jobs
}
