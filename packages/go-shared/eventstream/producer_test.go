package eventstream

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

func TestNewProducerRejectsNoBrokers(t *testing.T) {
	_, err := NewProducer(Config{})
	if err != ErrNoBrokers {
		t.Fatalf(
			"err = %v, want ErrNoBrokers",
			err,
		)
	}
}

func TestProducerAcceptsLoggerConfig(t *testing.T) {
	producer, err := NewProducer(Config{
		Brokers:  []string{"localhost:9092"},
		ClientID: "test",
		Logger:   slog.Default(),
	})
	if err != nil {
		t.Fatalf(
			"new producer: %v",
			err,
		)
	}
	if producer.log == nil {
		t.Fatal("logger is nil")
	}
	if producer.clientID != "test" {
		t.Fatalf(
			"clientID = %q, want test",
			producer.clientID,
		)
	}
	if err := producer.Close(); err != nil {
		t.Fatalf(
			"close producer: %v",
			err,
		)
	}
}

func TestProducerStatsRecordSuccess(t *testing.T) {
	writer := &fakeWriter{}
	producer := newProducerWithWriter(
		Config{
			ClientID:     "test",
			WriteTimeout: time.Second,
			MaxAttempts:  3,
			RetryBackoff: time.Millisecond,
		},
		writer,
	)
	err := producer.WriteJSON(
		context.Background(),
		"topic",
		"key",
		map[string]string{"status": "ok"},
	)
	if err != nil {
		t.Fatalf(
			"WriteJSON returned error: %v",
			err,
		)
	}
	stats := producer.Stats()
	if stats.WriteCallsTotal != 1 {
		t.Fatalf(
			"write calls = %d, want 1",
			stats.WriteCallsTotal,
		)
	}
	if stats.WriteAttemptsTotal != 1 {
		t.Fatalf(
			"write attempts = %d, want 1",
			stats.WriteAttemptsTotal,
		)
	}
	if stats.WriteSuccessTotal != 1 {
		t.Fatalf(
			"write success = %d, want 1",
			stats.WriteSuccessTotal,
		)
	}
	if stats.BytesWrittenTotal == 0 {
		t.Fatal("bytes written is 0")
	}
	if stats.LastSuccessAt == "" {
		t.Fatal("last success timestamp is empty")
	}
}

func TestProducerWritesBatchInOneCall(t *testing.T) {
	writer := &fakeWriter{}
	producer := newProducerWithWriter(
		Config{
			ClientID:     "test",
			WriteTimeout: time.Second,
			MaxAttempts:  1,
		},
		writer,
	)
	err := producer.WriteJSONBatch(
		context.Background(),
		"clicks",
		[]JSONEvent{
			{Key: "clk_1", Value: map[string]string{"click_id": "clk_1"}},
			{Key: "clk_2", Value: map[string]string{"click_id": "clk_2"}},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if writer.calls != 1 || len(writer.messages) != 2 ||
		string(writer.messages[0].Key) != "clk_1" ||
		string(writer.messages[1].Key) != "clk_2" {
		t.Fatalf("unexpected batch: calls=%d messages=%#v", writer.calls, writer.messages)
	}
}

func TestProducerRetriesOnlyFailedBatchMessages(t *testing.T) {
	writer := &fakeWriter{
		writeErrors: []error{
			kafka.WriteErrors{nil, errors.New("temporary failure")},
			nil,
		},
	}
	producer := newProducerWithWriter(
		Config{
			ClientID:     "test",
			WriteTimeout: time.Second,
			MaxAttempts:  2,
			RetryBackoff: time.Millisecond,
		},
		writer,
	)
	err := producer.WriteJSONBatch(
		context.Background(),
		"clicks",
		[]JSONEvent{
			{Key: "clk_1", Value: 1},
			{Key: "clk_2", Value: 2},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(writer.batches) != 2 || len(writer.batches[0]) != 2 ||
		len(writer.batches[1]) != 1 || string(writer.batches[1][0].Key) != "clk_2" {
		t.Fatalf("unexpected retry batches: %#v", writer.batches)
	}
}

func TestProducerStatsRecordRetriesAndFailure(t *testing.T) {
	writeErr := errors.New("broker unavailable")
	producer := newProducerWithWriter(
		Config{
			ClientID:     "test",
			WriteTimeout: time.Second,
			MaxAttempts:  3,
			RetryBackoff: time.Millisecond,
		},
		&fakeWriter{err: writeErr},
	)
	err := producer.WriteJSON(
		context.Background(),
		"topic",
		"key",
		map[string]string{"status": "ok"},
	)
	if !errors.Is(
		err,
		writeErr,
	) {
		t.Fatalf(
			"err = %v, want %v",
			err,
			writeErr,
		)
	}
	stats := producer.Stats()
	if stats.WriteAttemptsTotal != 3 {
		t.Fatalf(
			"write attempts = %d, want 3",
			stats.WriteAttemptsTotal,
		)
	}
	if stats.WriteRetryTotal != 2 {
		t.Fatalf(
			"write retry = %d, want 2",
			stats.WriteRetryTotal,
		)
	}
	if stats.WriteFailureTotal != 1 {
		t.Fatalf(
			"write failure = %d, want 1",
			stats.WriteFailureTotal,
		)
	}
	if stats.LastFailureAt == "" {
		t.Fatal("last failure timestamp is empty")
	}
	if stats.LastError != writeErr.Error() {
		t.Fatalf(
			"last error = %q, want %q",
			stats.LastError,
			writeErr.Error(),
		)
	}
}

func TestProducerStatsRecordMarshalFailure(t *testing.T) {
	producer := newProducerWithWriter(
		Config{
			ClientID:     "test",
			WriteTimeout: time.Second,
			MaxAttempts:  1,
			RetryBackoff: time.Millisecond,
		},
		&fakeWriter{},
	)
	err := producer.WriteJSON(
		context.Background(),
		"topic",
		"key",
		make(chan string),
	)
	if err == nil {
		t.Fatal("WriteJSON expected error")
	}
	stats := producer.Stats()
	if stats.MarshalFailureTotal != 1 {
		t.Fatalf(
			"marshal failure = %d, want 1",
			stats.MarshalFailureTotal,
		)
	}
	if stats.WriteAttemptsTotal != 0 {
		t.Fatalf(
			"write attempts = %d, want 0",
			stats.WriteAttemptsTotal,
		)
	}
}

type fakeWriter struct {
	err         error
	writeErrors []error
	calls       int
	messages    []kafka.Message
	batches     [][]kafka.Message
}

func (w *fakeWriter) WriteMessages(
	ctx context.Context,
	msgs ...kafka.Message,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	w.calls++
	w.messages = append(w.messages, msgs...)
	w.batches = append(w.batches, append([]kafka.Message(nil), msgs...))
	if len(w.writeErrors) >= w.calls {
		return w.writeErrors[w.calls-1]
	}
	return w.err
}

func (w *fakeWriter) Close() error {
	return nil
}
