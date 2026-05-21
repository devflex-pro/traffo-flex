package postbacklogs

import (
	"context"
	"encoding/json"

	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoLogger struct {
	collection *mongo.Collection
}

func NewMongoLogger(db *mongo.Database) *MongoLogger {
	return &MongoLogger{
		collection: db.Collection("postback_logs"),
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
	if _, err := l.collection.InsertOne(
		ctx,
		doc,
	); err != nil {
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
