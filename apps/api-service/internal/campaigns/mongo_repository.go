package campaigns

import (
	"context"
	"errors"
	"time"

	"github.com/devflex/traffoflex/apps/api-service/internal/mongostore"
	"github.com/devflex/traffoflex/apps/api-service/internal/scope"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoRepository struct {
	store      *mongostore.Store[models.Campaign]
	collection *mongo.Collection
	sources    *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		store: mongostore.New[models.Campaign](
			db.Collection("campaigns"),
			ErrInvalidInput,
			ErrNotFound,
		),
		collection: db.Collection("campaigns"),
		sources:    db.Collection("traffic_sources"),
	}
}

func (r *MongoRepository) List(ctx context.Context) (
	[]models.Campaign,
	error,
) {
	return r.store.List(ctx)
}

func (r *MongoRepository) Get(
	ctx context.Context,
	id string,
) (
	models.Campaign,
	error,
) {
	return r.store.Get(
		ctx,
		id,
	)
}

func (r *MongoRepository) Create(
	ctx context.Context,
	campaign models.Campaign,
) (
	models.Campaign,
	error,
) {
	if err := r.validateSource(ctx, campaign.TrafficSourceID); err != nil {
		return models.Campaign{}, err
	}
	return r.store.Create(
		ctx,
		campaign.ID,
		campaign,
	)
}

func (r *MongoRepository) Update(
	ctx context.Context,
	campaign models.Campaign,
) (
	models.Campaign,
	error,
) {
	if err := r.validateSource(ctx, campaign.TrafficSourceID); err != nil {
		return models.Campaign{}, err
	}
	return r.store.Update(
		ctx,
		campaign.ID,
		campaign,
	)
}

func (r *MongoRepository) UpdateTrackingParams(
	ctx context.Context,
	id string,
	params []models.TrackingParam,
) (
	models.Campaign,
	error,
) {
	filter := bson.M{"id": id}
	if ownerID := scope.OwnerID(ctx); ownerID != "" {
		filter["owner_id"] = ownerID
	}
	result, err := r.collection.UpdateOne(
		ctx,
		filter,
		bson.M{"$set": bson.M{
			"tracking_params": params,
			"updated_at":      time.Now().UTC(),
		}},
	)
	if err != nil {
		return models.Campaign{}, err
	}
	if result.MatchedCount == 0 {
		return models.Campaign{}, ErrNotFound
	}
	return r.store.Get(
		ctx,
		id,
	)
}

func (r *MongoRepository) validateSource(ctx context.Context, sourceID string) error {
	ownerID := scope.OwnerID(ctx)
	if ownerID == "" || sourceID == "" {
		return nil
	}
	count, err := r.sources.CountDocuments(
		ctx,
		bson.M{"id": sourceID, "owner_id": ownerID},
	)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.Join(ErrInvalidInput, errors.New("traffic source is unavailable"))
	}
	return nil
}

func (r *MongoRepository) Delete(
	ctx context.Context,
	id string,
) error {
	return r.store.Delete(
		ctx,
		id,
	)
}
