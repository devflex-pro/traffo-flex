package conversions

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/devflex/traffoflex/apps/postback-service/internal/normalize"
	"github.com/devflex/traffoflex/packages/go-shared/models"
)

func testConversion() normalize.Conversion {
	return normalize.Conversion{
		NetworkID:     "net_1",
		ClickID:       "clk_1",
		TransactionID: "tx_1",
		EventType:     "conversion",
		Status:        "approved",
		Payout:        10,
		Currency:      "USD",
	}
}

func TestServiceProcessIsIdempotent(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(repo)
	first, created, err := service.Process(
		context.Background(),
		testConversion(),
	)
	if err != nil || !created || first.ConversionID == "" {
		t.Fatalf("first Process = %+v, %v, %v", first, created, err)
	}
	second, created, err := service.Process(
		context.Background(),
		testConversion(),
	)
	if err != nil || created || second.ConversionID != first.ConversionID {
		t.Fatalf("duplicate Process = %+v, %v, %v", second, created, err)
	}
	if len(repo.Events()) != 1 {
		t.Fatalf("conversions = %d, want 1", len(repo.Events()))
	}
	pending, err := repo.Pending(context.Background(), 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending = %v, %v", pending, err)
	}
}

func TestSameNetworkTransactionBelongsToSeparateUsers(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(repo)
	firstInput := testConversion()
	firstInput.OwnerID = "usr_1"
	secondInput := testConversion()
	secondInput.OwnerID = "usr_2"
	first, firstCreated, err := service.Process(
		context.Background(),
		firstInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	second, secondCreated, err := service.Process(
		context.Background(),
		secondInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !firstCreated || !secondCreated || first.ConversionID == second.ConversionID {
		t.Fatalf(
			"created = %t/%t, IDs = %s/%s",
			firstCreated,
			secondCreated,
			first.ConversionID,
			second.ConversionID,
		)
	}
}

func TestConcurrentDuplicateCreatesOneConversion(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(repo)
	var pending sync.WaitGroup
	createdCount := make(chan bool, 20)
	for index := 0; index < 20; index++ {
		pending.Add(1)
		go func() {
			defer pending.Done()
			_, created, err := service.Process(
				context.Background(),
				testConversion(),
			)
			if err != nil {
				t.Errorf("Process: %v", err)
			}
			createdCount <- created
		}()
	}
	pending.Wait()
	close(createdCount)
	createdTotal := 0
	for created := range createdCount {
		if created {
			createdTotal++
		}
	}
	if createdTotal != 1 || len(repo.Events()) != 1 {
		t.Fatalf("created = %d, stored = %d", createdTotal, len(repo.Events()))
	}
}

type flakySink struct {
	fail   bool
	events []models.ConversionEvent
}

func (s *flakySink) Write(
	ctx context.Context,
	event models.ConversionEvent,
) error {
	if s.fail {
		return errors.New("redpanda unavailable")
	}
	s.events = append(s.events, event)
	return nil
}

func TestWorkerRetriesPendingConversion(t *testing.T) {
	repo := NewMemoryRepository()
	event, created, err := NewService(repo).Process(
		context.Background(),
		testConversion(),
	)
	if err != nil || !created {
		t.Fatalf("Process = %v, %v", created, err)
	}
	sink := &flakySink{fail: true}
	worker := NewWorker(
		repo,
		sink,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		time.Second,
	)
	worker.RunOnce(context.Background())
	pending, err := repo.Pending(context.Background(), 10)
	if err != nil || len(pending) != 1 {
		t.Fatalf("lost pending conversion: %v, %v", pending, err)
	}
	sink.fail = false
	worker.RunOnce(context.Background())
	pending, err = repo.Pending(context.Background(), 10)
	if err != nil || len(pending) != 0 || len(sink.events) != 1 || sink.events[0].ConversionID != event.ConversionID {
		t.Fatalf("delivery state = %v, %v, events=%v", pending, err, sink.events)
	}
}
