package postbacklogs

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/devflex/traffoflex/apps/api-service/internal/scope"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoRepository struct {
	collection *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		collection: db.Collection("postback_logs"),
	}
}

func (r *MongoRepository) List(
	ctx context.Context,
	query Query,
) (
	Report,
	error,
) {
	filter := buildFilter(query)
	if ownerID := scope.OwnerID(ctx); ownerID != "" {
		filter["owner_id"] = ownerID
	}
	direction := -1
	if query.Order == "asc" {
		direction = 1
	}
	cursor, err := r.collection.Aggregate(
		ctx,
		mongo.Pipeline{
			{{Key: "$match", Value: filter}},
			{{Key: "$set", Value: bson.M{"_log_time": logTimestamp()}}},
			{{Key: "$facet", Value: bson.M{
				"rows": mongo.Pipeline{
					{{Key: "$sort", Value: bson.D{{Key: "_log_time", Value: direction}, {Key: "_id", Value: direction}}}},
					{{Key: "$skip", Value: query.Offset}},
					{{Key: "$limit", Value: query.Limit}},
				},
				"totals": mongo.Pipeline{{{Key: "$count", Value: "count"}}},
			}}},
		},
	)
	if err != nil {
		return Report{}, err
	}
	var results []struct {
		Rows   []bson.M `bson:"rows"`
		Totals []struct {
			Count int `bson:"count"`
		} `bson:"totals"`
	}
	if err := cursor.All(
		ctx,
		&results,
	); err != nil {
		return Report{}, err
	}
	rows := make([]Row, 0)
	total := 0
	if len(results) == 0 {
		return Report{Items: rows, Filters: query.Filters(), Limit: query.Limit, Offset: query.Offset}, nil
	}
	if len(results[0].Totals) > 0 {
		total = results[0].Totals[0].Count
	}
	for _, raw := range results[0].Rows {
		row, err := decodeRow(raw)
		if err != nil {
			return Report{}, err
		}
		rows = append(
			rows,
			row,
		)
	}
	return Report{
		Items:   rows,
		Filters: query.Filters(),
		Limit:   query.Limit,
		Offset:  query.Offset,
		Total:   total,
	}, nil
}

func buildFilter(query Query) bson.M {
	filter := bson.M{}
	if query.ID != "" {
		filter["$or"] = bson.A{bson.M{"postback_id": query.ID}, bson.M{"click_id": query.ID}, bson.M{"transaction_id": query.ID}}
	}
	if !query.From.IsZero() || !query.To.IsZero() {
		conditions := bson.A{bson.M{"$ne": bson.A{logTimestamp(), nil}}}
		if !query.From.IsZero() {
			conditions = append(
				conditions,
				bson.M{"$gte": bson.A{logTimestamp(), query.From}},
			)
		}
		if !query.To.IsZero() {
			conditions = append(
				conditions,
				bson.M{"$lte": bson.A{logTimestamp(), query.To}},
			)
		}
		filter["$expr"] = bson.M{"$and": conditions}
	}
	if query.NetworkID != "" {
		filter["network_id"] = query.NetworkID
	}
	if query.ClickID != "" {
		filter["click_id"] = query.ClickID
	}
	if query.TransactionID != "" {
		filter["transaction_id"] = query.TransactionID
	}
	if query.Status != "" {
		filter["status"] = query.Status
	}
	return filter
}

// Logs use RFC3339 strings; accepting BSON dates also avoids a data migration.
func logTimestamp() bson.M {
	return bson.M{"$convert": bson.M{"input": "$created_at", "to": "date", "onError": nil, "onNull": nil}}
}

func decodeRow(doc bson.M) (
	Row,
	error,
) {
	delete(
		doc,
		"_id",
	)
	payload, err := json.Marshal(doc)
	if err != nil {
		return Row{}, err
	}
	var row Row
	if err := json.Unmarshal(
		payload,
		&row,
	); err != nil {
		return Row{}, err
	}
	if row.RawPayload == nil {
		row.RawPayload = map[string]string{}
	}
	// Derive display values from the original request so historical logs work
	// without rewriting data or changing the conversion/analytics event schema.
	for _, key := range []string{"payout", "sum", "amount", "revenue"} {
		raw := row.RawPayload[key]
		if raw == "" {
			continue
		}
		payout, err := strconv.ParseFloat(
			raw,
			64,
		)
		if err == nil && !math.IsNaN(payout) && !math.IsInf(
			payout,
			0,
		) {
			row.Payout = &payout
		}
		break
	}
	row.Currency = strings.ToUpper(strings.TrimSpace(row.RawPayload["currency"]))
	if row.Currency == "" {
		row.Currency = "USD"
	}
	return row, nil
}
