package conversions

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoRepository struct {
	conversions *mongo.Collection
	dedupe      *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		conversions: db.Collection("conversions"),
		dedupe:      db.Collection("conversion_dedupe"),
	}
}

func (r *MongoRepository) Create(
	ctx context.Context,
	event models.ConversionEvent,
) (
	models.ConversionEvent,
	error,
) {
	doc, err := encodeDocument(event)
	if err != nil {
		return models.ConversionEvent{}, err
	}
	doc["_id"] = event.ConversionID
	if _, err := r.conversions.InsertOne(
		ctx,
		doc,
	); err != nil {
		return models.ConversionEvent{}, err
	}
	return event, nil
}

func (r *MongoRepository) ExistsDedupe(
	ctx context.Context,
	key DedupeKey,
) (
	bool,
	error,
) {
	res := r.dedupe.FindOne(
		ctx,
		bson.M{"_id": key.String()},
	)
	if err := res.Err(); err != nil {
		if errors.Is(
			err,
			mongo.ErrNoDocuments,
		) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *MongoRepository) MarkDedupe(
	ctx context.Context,
	key DedupeKey,
) error {
	_, err := r.dedupe.InsertOne(
		ctx,
		bson.M{
			"_id":            key.String(),
			"network_id":     key.NetworkID,
			"transaction_id": key.TransactionID,
			"event_type":     key.EventType,
			"click_id":       key.ClickID,
		},
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func encodeDocument(item any) (
	bson.M,
	error,
) {
	payload, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	var doc bson.M
	if err := json.Unmarshal(
		payload,
		&doc,
	); err != nil {
		return nil, err
	}
	return doc, nil
}
