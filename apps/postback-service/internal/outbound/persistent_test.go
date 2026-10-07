package outbound

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devflex/traffoflex/apps/postback-service/internal/conversions"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type roundTripFunc func(*http.Request) (
	*http.Response,
	error,
)

func (f roundTripFunc) RoundTrip(request *http.Request) (
	*http.Response,
	error,
) {
	return f(request)
}

func response(
	code int,
	body string,
) *http.Response {
	return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestDeliverySelectsOnlyOwnScopeAndApprovedEvents(t *testing.T) {
	now := time.Now()
	event := models.ConversionEvent{OwnerID: "owner", ClickID: "internal", CreatedAt: now, Payout: 1.25}
	click := Click{OwnerID: "owner", SourceID: "src", CampaignID: "cmp", SourceClickID: "external", Sub1: "zone"}
	template := models.PostbackTemplate{OwnerID: "owner", Direction: "outgoing", Enabled: true, SourceID: "src", CampaignID: "cmp", CreatedAt: now.Add(-time.Second)}
	if !Matches(
		template,
		event,
		click,
	) {
		t.Fatal("own matching template was rejected")
	}
	for _, change := range []func(*models.PostbackTemplate){func(item *models.PostbackTemplate) { item.OwnerID = "another" }, func(item *models.PostbackTemplate) { item.SourceID = "other" }, func(item *models.PostbackTemplate) { item.CampaignID = "other" }, func(item *models.PostbackTemplate) { item.Enabled = false }, func(item *models.PostbackTemplate) { item.CreatedAt = now.Add(time.Second) }} {
		copy := template
		change(&copy)
		if Matches(
			copy,
			event,
			click,
		) {
			t.Fatal("mismatched template was accepted")
		}
	}
	values := Values(
		event,
		click,
	)
	if values["source_click_id"] != "external" || values["cid"] != "internal" || values["sum"] != "1.25" || values["s1"] != "zone" {
		t.Fatalf(
			"incorrect values: %v",
			values,
		)
	}
	if Approved("pending") || Approved("rejected") || !Approved("APPROVED") {
		t.Fatal("incorrect approved-status filter")
	}
}

func TestSendUsesStableKeyAndRejectsRedirects(t *testing.T) {
	client := NewPublicClient()
	requests := 0
	client.Transport = roundTripFunc(func(request *http.Request) (
		*http.Response,
		error,
	) {
		requests++
		if request.Method != "GET" || request.Header.Get("Idempotency-Key") != "event:template" {
			t.Fatal("missing stable delivery key")
		}
		result := response(
			http.StatusFound,
			"",
		)
		result.Header.Set(
			"Location",
			"https://other.example/pb",
		)
		return result, nil
	})
	code, err := Send(
		context.Background(),
		client,
		"https://receiver.example/pb?event=event",
		"event:template",
	)
	if err == nil || code != http.StatusFound || requests != 1 {
		t.Fatalf(
			"code=%d error=%v requests=%d",
			code,
			err,
			requests,
		)
	}
	if _, err := publicDial(
		context.Background(),
		"tcp",
		"127.0.0.1:80",
	); err == nil {
		t.Fatal("public transport accepted loopback")
	}
}

func TestPersistentDeliveryRecovery(t *testing.T) {
	uri := os.Getenv("OUTBOUND_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set OUTBOUND_TEST_MONGO_URI to an isolated MongoDB to test durable delivery")
	}
	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	db := client.Database("traffoflex_outbound_test")
	t.Cleanup(func() {
		if err := db.Drop(context.Background()); err != nil {
			t.Error(err)
		}
		if err := client.Disconnect(context.Background()); err != nil {
			t.Error(err)
		}
	})
	worker, err := NewPersistentWorker(
		ctx,
		db,
		"http://clickhouse.invalid",
		slog.New(slog.NewTextHandler(
			io.Discard,
			nil,
		)),
	)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	template := bson.M{"id": "template", "owner_id": "owner", "direction": "outgoing", "enabled": true, "url": "https://receiver.example/pb?click={source_click_id}&event={conversion_id}&sum={sum}", "created_at": now.Add(-time.Hour), "source_id": "src"}
	if _, err := db.Collection("postback_templates").InsertOne(
		ctx,
		template,
	); err != nil {
		t.Fatal(err)
	}
	event := models.ConversionEvent{OwnerID: "owner", ConversionID: "event", ClickID: "internal", TransactionID: "transaction", NetworkID: "network", Status: "approved", Payout: 1.25, Currency: "USD", CreatedAt: now}
	repo := conversions.NewMongoRepository(db)
	if _, _, err := repo.CreateOrGet(
		ctx,
		event,
	); err != nil {
		t.Fatal(err)
	}
	// A legacy conversion has no outbound state and must never be replayed.
	if _, err := db.Collection("conversions").InsertOne(
		ctx,
		bson.M{"_id": "legacy", "owner_id": "owner"},
	); err != nil {
		t.Fatal(err)
	}
	worker.lookupClient.Transport = roundTripFunc(func(request *http.Request) (
		*http.Response,
		error,
	) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		if !strings.Contains(
			string(body),
			"owner_id = 'owner'",
		) {
			t.Fatal("unscoped click lookup")
		}
		return response(
			200,
			`{"owner_id":"owner","source_id":"src","source_click_id":"external&value","campaign_id":"cmp"}`,
		), nil
	})
	if err := worker.plan(ctx); err != nil {
		t.Fatal(err)
	}
	// Replanning after an uncertain checkpoint must not enqueue a second notification.
	if err := worker.planEvent(
		ctx,
		event,
	); err != nil {
		t.Fatal(err)
	}
	count, err := db.Collection("outbound_postbacks").CountDocuments(
		ctx,
		bson.M{},
	)
	if err != nil || count != 1 {
		t.Fatalf(
			"jobs=%d error=%v",
			count,
			err,
		)
	}
	attempts := 0
	worker.client.Transport = roundTripFunc(func(request *http.Request) (
		*http.Response,
		error,
	) {
		attempts++
		if request.URL.Query().Get("click") != "external&value" || request.URL.Query().Get("sum") != "1.25" {
			t.Fatal("incorrect outbound attribution")
		}
		if attempts == 1 {
			return response(
				503,
				"unavailable",
			), nil
		}
		return response(
			204,
			"",
		), nil
	})
	if _, err := worker.Retry(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Collection("outbound_postbacks").UpdateOne(
		ctx,
		bson.M{"_id": "event:template"},
		bson.M{"$set": bson.M{"status": "sending", "lease_until": now.Add(-time.Minute)}},
	); err != nil {
		t.Fatal(err)
	}
	// Simulate a new worker after a crash: leases and retries survive process memory.
	restarted, err := NewPersistentWorker(
		ctx,
		db,
		"http://clickhouse.invalid",
		worker.log,
	)
	if err != nil {
		t.Fatal(err)
	}
	restarted.client = worker.client
	if delivered, err := restarted.Retry(ctx); err != nil || delivered != 1 {
		t.Fatalf(
			"delivered=%d error=%v",
			delivered,
			err,
		)
	}
	if _, err := restarted.Retry(ctx); err != nil {
		t.Fatal(err)
	}
	var job bson.M
	if err := db.Collection("outbound_postbacks").FindOne(
		ctx,
		bson.M{"_id": "event:template"},
	).Decode(&job); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	if job["status"] != "delivered" || attempts != 2 {
		t.Fatalf(
			"job=%s requests=%d",
			encoded,
			attempts,
		)
	}
}
