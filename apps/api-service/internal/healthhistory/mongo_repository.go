package healthhistory

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/devflex/traffoflex/apps/api-service/internal/scope"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoRepository struct {
	collection *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		collection: db.Collection("destination_health_history"),
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
	total, err := r.collection.CountDocuments(
		ctx,
		filter,
	)
	if err != nil {
		return Report{}, err
	}
	cursor, err := r.collection.Find(
		ctx,
		filter,
		options.Find().
			SetSort(bson.D{{Key: "checked_at", Value: -1}}).
			SetSkip(int64(query.Offset)).
			SetLimit(int64(query.Limit)),
	)
	if err != nil {
		return Report{}, err
	}
	rows := make([]Row, 0)
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			if closeErr := cursor.Close(ctx); closeErr != nil {
				return Report{}, errors.Join(
					err,
					closeErr,
				)
			}
			return Report{}, err
		}
		row, err := decodeRow(raw)
		if err != nil {
			if closeErr := cursor.Close(ctx); closeErr != nil {
				return Report{}, errors.Join(
					err,
					closeErr,
				)
			}
			return Report{}, err
		}
		rows = append(
			rows,
			row,
		)
	}
	if err := cursor.Err(); err != nil {
		if closeErr := cursor.Close(ctx); closeErr != nil {
			return Report{}, errors.Join(
				err,
				closeErr,
			)
		}
		return Report{}, err
	}
	if err := cursor.Close(ctx); err != nil {
		return Report{}, err
	}
	return Report{
		Items:   rows,
		Filters: query.Filters(),
		Limit:   query.Limit,
		Offset:  query.Offset,
		Total:   int(total),
	}, nil
}

func buildFilter(query Query) bson.M {
	filter := bson.M{}
	if query.DestinationID != "" {
		filter["destination_id"] = query.DestinationID
	}
	if query.Current != "" {
		filter["current"] = query.Current
	}
	return filter
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
	return row, nil
}
