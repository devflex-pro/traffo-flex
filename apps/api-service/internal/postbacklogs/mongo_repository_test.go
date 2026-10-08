package postbacklogs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/devflex/traffoflex/apps/api-service/internal/scope"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestMongoLogPeriodSearchSortingAndIsolation(t *testing.T) {
	uri := os.Getenv("POSTBACK_LOG_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set POSTBACK_LOG_TEST_MONGO_URI to an isolated MongoDB")
	}
	ctx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	db := client.Database("traffoflex_log_filters_test")
	t.Cleanup(func() {
		if err := db.Drop(context.Background()); err != nil {
			t.Error(err)
		}
		if err := client.Disconnect(context.Background()); err != nil {
			t.Error(err)
		}
	})
	from := time.Date(
		2026,
		10,
		8,
		0,
		0,
		0,
		0,
		time.UTC,
	)
	to := from.Add(24*time.Hour - time.Millisecond)
	if _, err := db.Collection("postback_logs").InsertMany(
		ctx,
		[]any{
			bson.M{"_id": "pb-early", "postback_id": "pb-early", "owner_id": "owner", "click_id": "shared", "transaction_id": "tx-early", "created_at": "2026-10-08T00:00:00Z"},
			bson.M{"_id": "pb-fraction", "postback_id": "pb-fraction", "owner_id": "owner", "click_id": "shared", "transaction_id": "tx-fraction", "created_at": "2026-10-08T00:00:00.5Z"},
			bson.M{"_id": "pb-end", "postback_id": "pb-end", "owner_id": "owner", "created_at": to},
			bson.M{"_id": "pb-prev", "postback_id": "pb-prev", "owner_id": "owner", "click_id": "shared", "created_at": "2026-10-07T23:59:59.999Z"},
			bson.M{"_id": "pb-other", "postback_id": "pb-other", "owner_id": "other", "click_id": "shared", "created_at": "2026-10-08T12:00:00Z"},
		},
	); err != nil {
		t.Fatal(err)
	}
	ctx = scope.WithValue(
		ctx,
		scope.Value{ActorID: "owner", OwnerID: "owner"},
	)
	repo := NewMongoRepository(db)
	query := Query{From: from, To: to, Limit: 100, Order: "asc"}
	report, err := repo.List(
		ctx,
		query,
	)
	if err != nil {
		t.Fatal(err)
	}
	if report.Total != 3 || len(report.Items) != 3 || report.Items[0].PostbackID != "pb-early" || report.Items[1].PostbackID != "pb-fraction" || report.Items[2].PostbackID != "pb-end" {
		t.Fatalf(
			"period/order/isolation failed: %+v",
			report,
		)
	}
	query.Order, query.Offset, query.Limit = "desc", 1, 1
	report, err = repo.List(
		ctx,
		query,
	)
	if err != nil {
		t.Fatal(err)
	}
	if report.Total != 3 || len(report.Items) != 1 || report.Items[0].PostbackID != "pb-fraction" {
		t.Fatalf(
			"sorted pagination failed: %+v",
			report,
		)
	}
	query.Offset, query.Limit = 0, 100
	for id, expected := range map[string]int{"shared": 2, "tx-early": 1, "pb-end": 1, "pb-other": 0} {
		query.ID = id
		report, err = repo.List(
			ctx,
			query,
		)
		if err != nil {
			t.Fatal(err)
		}
		if report.Total != expected || len(report.Items) != expected {
			t.Fatalf(
				"ID %q: got %+v, want %d own matches in period",
				id,
				report,
				expected,
			)
		}
	}
}
