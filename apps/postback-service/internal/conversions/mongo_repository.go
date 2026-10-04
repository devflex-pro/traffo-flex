package conversions

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

type MongoRepository struct {
	conversions *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		conversions: db.Collection(
			"conversions",
			options.Collection().SetWriteConcern(writeconcern.Journaled()),
		),
	}
}

func (r *MongoRepository) CreateOrGet(
	ctx context.Context,
	event models.ConversionEvent,
) (
	models.ConversionEvent,
	bool,
	error,
) {
	doc, err := encodeDocument(event)
	if err != nil {
		return models.ConversionEvent{}, false, err
	}
	doc["_id"] = event.ConversionID
	doc["delivery_status"] = "pending"
	doc["attribution_status"] = "pending"
	doc["attribution_attempts"] = 0
	doc["attribution_next_attempt_at"] = time.Now().UTC()
	if _, err := r.conversions.InsertOne(
		ctx,
		doc,
	); err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return models.ConversionEvent{}, false, err
		}
		var existing bson.M
		findErr := r.conversions.FindOne(
			ctx,
			bson.M{"owner_id": event.OwnerID, "network_id": event.NetworkID, "transaction_id": event.TransactionID},
		).Decode(&existing)
		if findErr != nil {
			return models.ConversionEvent{}, false, errors.Join(err, findErr)
		}
		found, decodeErr := decodeDocument(existing)
		return found, false, decodeErr
	}
	return event, true, nil
}

func (r *MongoRepository) Pending(
	ctx context.Context,
	limit int,
) (
	[]models.ConversionEvent,
	error,
) {
	cursor, err := r.conversions.Find(
		ctx,
		bson.M{"delivery_status": "pending"},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	var events []models.ConversionEvent
	for _, doc := range docs {
		event, err := decodeDocument(doc)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (r *MongoRepository) MarkDelivered(
	ctx context.Context,
	conversionID string,
) error {
	result, err := r.conversions.UpdateOne(
		ctx,
		bson.M{"_id": conversionID, "delivery_status": "pending"},
		bson.M{"$set": bson.M{"delivery_status": "delivered", "delivered_at": time.Now().UTC()}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return errors.New("pending conversion not found")
	}
	return nil
}

func (r *MongoRepository) PendingAttribution(
	ctx context.Context,
	limit int,
) ([]AttributionPending, error) {
	cursor, err := r.conversions.Find(
		ctx,
		bson.M{
			"attribution_status":          "pending",
			"attribution_next_attempt_at": bson.M{"$lte": time.Now().UTC()},
		},
		options.Find().SetSort(bson.D{{Key: "attribution_next_attempt_at", Value: 1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	result := make([]AttributionPending, 0, len(docs))
	for _, doc := range docs {
		event, err := decodeDocument(doc)
		if err != nil {
			return nil, err
		}
		attempts, ok := doc["attribution_attempts"].(int32)
		if !ok {
			return nil, errors.New("invalid attribution_attempts")
		}
		result = append(result, AttributionPending{Event: event, Attempts: int(attempts)})
	}
	return result, nil
}

func (r *MongoRepository) DeferAttribution(
	ctx context.Context,
	conversionID string,
	attempts int,
	next time.Time,
) error {
	result, err := r.conversions.UpdateOne(
		ctx,
		bson.M{"_id": conversionID, "attribution_status": "pending"},
		bson.M{"$set": bson.M{"attribution_attempts": attempts, "attribution_next_attempt_at": next}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return errors.New("pending attribution not found")
	}
	return nil
}

func (r *MongoRepository) MarkAttributed(
	ctx context.Context,
	conversionID string,
) error {
	result, err := r.conversions.UpdateOne(
		ctx,
		bson.M{"_id": conversionID, "attribution_status": "pending"},
		bson.M{"$set": bson.M{"attribution_status": "attributed", "attributed_at": time.Now().UTC()}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return errors.New("pending attribution not found")
	}
	return nil
}

func decodeDocument(doc bson.M) (models.ConversionEvent, error) {
	payload, err := json.Marshal(doc)
	if err != nil {
		return models.ConversionEvent{}, err
	}
	var event models.ConversionEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return models.ConversionEvent{}, err
	}
	return event, nil
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
