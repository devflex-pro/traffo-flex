package cache

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/devflex/traffoflex/apps/traffic-service/internal/healthstate"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoLoader struct {
	campaigns    *mongo.Collection
	streams      *mongo.Collection
	destinations *mongo.Collection
	db           *mongo.Database
}

func NewMongoLoader(db *mongo.Database) *MongoLoader {
	return &MongoLoader{
		campaigns:    db.Collection("campaigns"),
		streams:      db.Collection("streams"),
		destinations: db.Collection("destinations"),
		db:           db,
	}
}

func (l *MongoLoader) Load(ctx context.Context) (
	[]CampaignConfig,
	error,
) {
	campaigns, err := listDocuments[models.Campaign](
		ctx,
		l.campaigns,
		bson.M{"status": string(models.StatusActive)},
	)
	if err != nil {
		return nil, err
	}
	destinations, err := listDocuments[models.Destination](
		ctx,
		l.destinations,
		bson.M{},
	)
	if err != nil {
		return nil, err
	}
	healthStates, err := healthstate.LoadStates(
		ctx,
		l.db,
	)
	if err != nil {
		return nil, err
	}
	overlayDestinationHealth(
		destinations,
		healthStates,
	)

	configs := make(
		[]CampaignConfig,
		0,
		len(campaigns),
	)
	for _, campaign := range campaigns {
		streams, err := listDocuments[models.Stream](
			ctx,
			l.streams,
			bson.M{
				"campaign_id": campaign.ID,
				"status":      string(models.StatusActive),
			},
		)
		if err != nil {
			return nil, err
		}
		configs = append(
			configs,
			CampaignConfig{
				Campaign:     campaign,
				Streams:      streams,
				Destinations: destinations,
			},
		)
	}
	return configs, nil
}

func overlayDestinationHealth(
	destinations []models.Destination,
	states map[string]healthstate.State,
) {
	for index, destination := range destinations {
		state, ok := states[destination.ID]
		if !ok {
			continue
		}
		if state.Current.Valid() {
			destinations[index].HealthStatus = state.Current
			destinations[index].UpdatedAt = state.UpdatedAt
		}
	}
}

func listDocuments[T any](
	ctx context.Context,
	collection *mongo.Collection,
	filter bson.M,
) (
	[]T,
	error,
) {
	cursor, err := collection.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}

	var items []T
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
		item, err := decodeDocument[T](raw)
		if err != nil {
			if closeErr := cursor.Close(ctx); closeErr != nil {
				return nil, errors.Join(
					err,
					closeErr,
				)
			}
			return nil, err
		}
		items = append(
			items,
			item,
		)
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
	if items == nil {
		items = []T{}
	}
	return items, nil
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
