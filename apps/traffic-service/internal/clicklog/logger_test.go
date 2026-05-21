package clicklog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestAsyncLoggerWritesToSink(t *testing.T) {
	sink := NewMemorySink()
	logger := NewAsyncLogger(
		sink,
		1,
	)
	defer logger.Close()

	if err := logger.Log(
		context.Background(),
		models.ClickEvent{ClickID: "clk_1"},
	); err != nil {
		t.Fatalf(
			"Log returned error: %v",
			err,
		)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(sink.Events()) == 1 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("event was not written")
}

func TestAsyncLoggerReturnsQueueFull(t *testing.T) {
	logger := &AsyncLogger{
		sink: NewMemorySink(),
		queue: make(
			chan models.ClickEvent,
			1,
		),
		done: make(chan struct{}),
	}

	if err := logger.Log(
		context.Background(),
		models.ClickEvent{ClickID: "clk_1"},
	); err != nil {
		t.Fatalf(
			"first Log returned error: %v",
			err,
		)
	}
	if err := logger.Log(
		context.Background(),
		models.ClickEvent{ClickID: "clk_2"},
	); !errors.Is(
		err,
		ErrQueueFull,
	) {
		t.Fatalf(
			"second Log error = %v, want ErrQueueFull",
			err,
		)
	}
}
