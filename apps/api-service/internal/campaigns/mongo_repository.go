package campaigns

import (
	"context"
	"errors"

	"github.com/devflex/traffoflex/apps/api-service/internal/mongostore"
	"github.com/devflex/traffoflex/apps/api-service/internal/scope"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoRepository struct {
	store   *mongostore.Store[models.Campaign]
	sources *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		store: mongostore.New[models.Campaign](
			db.Collection("campaigns"),
			ErrInvalidInput,
			ErrNotFound,
		),
		sources: db.Collection("traffic_sources"),
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
