package postbacklogs

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

type MongoLogger struct {
	collection *mongo.Collection
}

func NewMongoLogger(db *mongo.Database) *MongoLogger {
	return &MongoLogger{
		collection: db.Collection(
			"postback_logs",
			options.Collection().SetWriteConcern(writeconcern.Journaled()),
		),
	}
}

func (l *MongoLogger) Log(
	ctx context.Context,
	event models.PostbackLogEvent,
) error {
	doc, err := encodeDocument(event)
	if err != nil {
		return err
	}
	doc["_id"] = event.PostbackID
	doc["delivery_status"] = "pending"
	if _, err := l.collection.InsertOne(
		ctx,
		doc,
	); err != nil {
		return err
	}
	return nil
}

func (l *MongoLogger) Pending(
	ctx context.Context,
	limit int,
) ([]models.PostbackLogEvent, error) {
	cursor, err := l.collection.Find(
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
	events := make([]models.PostbackLogEvent, 0, len(docs))
	for _, doc := range docs {
		payload, err := json.Marshal(doc)
		if err != nil {
			return nil, err
		}
		var event models.PostbackLogEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (l *MongoLogger) MarkDelivered(
	ctx context.Context,
	postbackID string,
) error {
	result, err := l.collection.UpdateOne(
		ctx,
		bson.M{"_id": postbackID, "delivery_status": "pending"},
		bson.M{"$set": bson.M{"delivery_status": "delivered", "delivered_at": time.Now().UTC()}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount != 1 {
		return errors.New("pending postback log not found")
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
