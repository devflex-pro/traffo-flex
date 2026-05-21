package conversions

import (
	"context"
	"errors"
	"testing"

	"github.com/devflex/traffoflex/apps/postback-service/internal/normalize"
)

func TestServiceProcessCreatesConversion(t *testing.T) {
	repo := NewMemoryRepository()
	sink := NewMemoryEventSink()
	service := NewServiceWithSink(
		repo,
		sink,
	)

	event, err := service.Process(context.Background(), normalize.Conversion{
		NetworkID:     "net_1",
		ClickID:       "clk_1",
		TransactionID: "tx_1",
		EventType:     "conversion",
		Status:        "approved",
		Payout:        10,
		Currency:      "USD",
	})
	if err != nil {
		t.Fatalf(
			"Process returned error: %v",
			err,
		)
	}
	if event.ConversionID == "" {
		t.Fatal("ConversionID is empty")
	}
	if len(repo.Events()) != 1 {
		t.Fatalf(
			"len(events) = %d, want 1",
			len(repo.Events()),
		)
	}
	if len(sink.Events()) != 1 {
		t.Fatalf(
			"len(sink events) = %d, want 1",
			len(sink.Events()),
		)
	}
}

func TestServiceProcessEnrichesConversionFromClickLookup(t *testing.T) {
	repo := NewMemoryRepository()
	sink := NewMemoryEventSink()
	service := NewServiceWithDependencies(
		repo,
		sink,
		staticClickLookup{
			info: ClickInfo{
				CampaignID:    "cmp_1",
				StreamID:      "str_1",
				DestinationID: "dst_1",
				SourceID:      "src_1",
			},
			found: true,
		},
	)

	event, err := service.Process(
		context.Background(),
		normalize.Conversion{
			NetworkID:     "net_1",
			ClickID:       "clk_1",
			TransactionID: "tx_1",
			EventType:     "conversion",
			Status:        "approved",
		},
	)
	if err != nil {
		t.Fatalf(
			"Process returned error: %v",
			err,
		)
	}
	if event.CampaignID != "cmp_1" {
		t.Fatalf(
			"CampaignID = %q, want cmp_1",
			event.CampaignID,
		)
	}
	if event.StreamID != "str_1" {
		t.Fatalf(
			"StreamID = %q, want str_1",
			event.StreamID,
		)
	}
	if event.DestinationID != "dst_1" {
		t.Fatalf(
			"DestinationID = %q, want dst_1",
			event.DestinationID,
		)
	}
	if event.SourceID != "src_1" {
		t.Fatalf(
			"SourceID = %q, want src_1",
			event.SourceID,
		)
	}
}

func TestServiceProcessRejectsDuplicate(t *testing.T) {
	service := NewService(NewMemoryRepository())
	conversion := normalize.Conversion{
		NetworkID:     "net_1",
		ClickID:       "clk_1",
		TransactionID: "tx_1",
		EventType:     "conversion",
	}

	if _, err := service.Process(
		context.Background(),
		conversion,
	); err != nil {
		t.Fatalf(
			"first Process returned error: %v",
			err,
		)
	}
	if _, err := service.Process(
		context.Background(),
		conversion,
	); !errors.Is(
		err,
		ErrDuplicate,
	) {
		t.Fatalf(
			"second Process error = %v, want ErrDuplicate",
			err,
		)
	}
}

type staticClickLookup struct {
	info  ClickInfo
	found bool
	err   error
}

func (l staticClickLookup) Find(
	context.Context,
	string,
) (
	ClickInfo,
	bool,
	error,
) {
	return l.info, l.found, l.err
}
