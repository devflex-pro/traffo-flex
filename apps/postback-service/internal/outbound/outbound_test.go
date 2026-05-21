package outbound

import (
	"context"
	"testing"

	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func TestRender(t *testing.T) {
	url, err := Render(Template{ID: "tpl_1", Enabled: true, URL: "https://tracker.example/pb?cid={click_id}&tx={transaction_id}"}, models.ConversionEvent{
		ClickID:       "clk_1",
		TransactionID: "tx_1",
	})
	if err != nil {
		t.Fatalf(
			"Render returned error: %v",
			err,
		)
	}
	if url != "https://tracker.example/pb?cid=clk_1&tx=tx_1" {
		t.Fatalf(
			"url = %q",
			url,
		)
	}
}

func TestServiceEnqueueForConversion(t *testing.T) {
	queue := NewMemoryQueue(10)
	service := NewService(queue, []Template{
		{ID: "tpl_1", Enabled: true, URL: "https://tracker.example/pb?cid={click_id}"},
	})

	if err := service.EnqueueForConversion(
		context.Background(),
		models.ConversionEvent{ClickID: "clk_1"},
	); err != nil {
		t.Fatalf(
			"EnqueueForConversion returned error: %v",
			err,
		)
	}
	if len(queue.Jobs()) != 1 {
		t.Fatalf(
			"len(jobs) = %d, want 1",
			len(queue.Jobs()),
		)
	}
}

func TestMemoryQueueFull(t *testing.T) {
	queue := NewMemoryQueue(1)
	if err := queue.Enqueue(
		context.Background(),
		Job{ID: "job_1"},
	); err != nil {
		t.Fatalf(
			"first Enqueue returned error: %v",
			err,
		)
	}
	if err := queue.Enqueue(
		context.Background(),
		Job{ID: "job_2"},
	); err == nil {
		t.Fatal("expected queue full error")
	}
}
