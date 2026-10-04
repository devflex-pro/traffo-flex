package healthstate

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type State struct {
	DestinationID string              `json:"destination_id"`
	OwnerID       string              `json:"owner_id,omitempty"`
	Previous      models.HealthStatus `json:"previous"`
	Current       models.HealthStatus `json:"current"`
	Error         string              `json:"error,omitempty"`
	CheckedAt     time.Time           `json:"checked_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

type Repository interface {
	Upsert(
		ctx context.Context,
		state State,
	) error
}

type MongoRepository struct {
	current *mongo.Collection
	history *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		current: db.Collection("destination_health"),
		history: db.Collection("destination_health_history"),
	}
}

func (r *MongoRepository) Upsert(
	ctx context.Context,
	state State,
) error {
	doc, err := encodeDocument(state)
	if err != nil {
		return err
	}
	doc["_id"] = state.DestinationID
	if _, err := r.current.UpdateOne(
		ctx,
		bson.M{"_id": state.DestinationID},
		bson.M{"$set": doc},
		options.UpdateOne().SetUpsert(true),
	); err != nil {
		return err
	}
	if state.Previous != state.Current {
		historyDoc, err := encodeDocument(state)
		if err != nil {
			return err
		}
		historyDoc["_id"] = bson.NewObjectID()
		if _, err := r.history.InsertOne(
			ctx,
			historyDoc,
		); err != nil {
			return err
		}
	}
	return nil
}

func LoadStates(
	ctx context.Context,
	db *mongo.Database,
) (
	map[string]State,
	error,
) {
	collection := db.Collection("destination_health")
	cursor, err := collection.Find(
		ctx,
		bson.M{},
	)
	if err != nil {
		return nil, err
	}

	states := make(map[string]State)
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			if closeErr := cursor.Close(ctx); closeErr != nil {
				return nil, errors.Join(
					err,
					closeErr,
				)
			}
			return nil, err
		}
		state, err := decodeDocument[State](raw)
		if err != nil {
			if closeErr := cursor.Close(ctx); closeErr != nil {
				return nil, errors.Join(
					err,
					closeErr,
				)
			}
			return nil, err
		}
		states[state.DestinationID] = state
	}
	if err := cursor.Err(); err != nil {
		if closeErr := cursor.Close(ctx); closeErr != nil {
			return nil, errors.Join(
				err,
				closeErr,
			)
		}
		return nil, err
	}
	if err := cursor.Close(ctx); err != nil {
		return nil, err
	}
	return states, nil
}

func LoadState(
	ctx context.Context,
	db *mongo.Database,
	destinationID string,
) (
	State,
	bool,
	error,
) {
	var raw bson.M
	err := db.Collection("destination_health").FindOne(
		ctx,
		bson.M{"_id": destinationID},
	).Decode(&raw)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return State{}, false, nil
	}
	if err != nil {
		return State{}, false, err
	}
	state, err := decodeDocument[State](raw)
	if err != nil {
		return State{}, false, err
	}
	return state, true, nil
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

func decodeDocument[T any](doc bson.M) (
	T,
	error,
) {
	var item T
	delete(
		doc,
		"_id",
	)
	payload, err := json.Marshal(doc)
	if err != nil {
		return item, err
	}
	if err := json.Unmarshal(
		payload,
		&item,
	); err != nil {
		return item, err
	}
	return item, nil
}
