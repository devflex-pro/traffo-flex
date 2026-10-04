package streams

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
	store        *mongostore.Store[models.Stream]
	campaigns    *mongo.Collection
	destinations *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		store: mongostore.New[models.Stream](
			db.Collection("streams"),
			ErrInvalidInput,
			ErrNotFound,
		),
		campaigns:    db.Collection("campaigns"),
		destinations: db.Collection("destinations"),
	}
}

func (r *MongoRepository) ListAll(ctx context.Context) (
	[]models.Stream,
	error,
) {
	return r.store.List(ctx)
}

func (r *MongoRepository) ListByCampaign(
	ctx context.Context,
	campaignID string,
) (
	[]models.Stream,
	error,
) {
	return r.store.ListByField(
		ctx,
		"campaign_id",
		campaignID,
	)
}

func (r *MongoRepository) Get(
	ctx context.Context,
	id string,
) (
	models.Stream,
	error,
) {
	return r.store.Get(
		ctx,
		id,
	)
}

func (r *MongoRepository) Create(
	ctx context.Context,
	stream models.Stream,
) (
	models.Stream,
	error,
) {
	if err := r.validateReferences(ctx, stream); err != nil {
		return models.Stream{}, err
	}
	return r.store.Create(
		ctx,
		stream.ID,
		stream,
	)
}

func (r *MongoRepository) Update(
	ctx context.Context,
	stream models.Stream,
) (
	models.Stream,
	error,
) {
	if err := r.validateReferences(ctx, stream); err != nil {
		return models.Stream{}, err
	}
	return r.store.Update(
		ctx,
		stream.ID,
		stream,
	)
}

func (r *MongoRepository) validateReferences(ctx context.Context, stream models.Stream) error {
	ownerID := scope.OwnerID(ctx)
	if ownerID == "" {
		return nil
	}
	count, err := r.campaigns.CountDocuments(
		ctx,
		bson.M{"id": stream.CampaignID, "owner_id": ownerID},
	)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.Join(ErrInvalidInput, errors.New("campaign is unavailable"))
	}
	for _, target := range stream.Distribution.Destinations {
		count, err := r.destinations.CountDocuments(
			ctx,
			bson.M{"id": target.DestinationID, "owner_id": ownerID},
		)
		if err != nil {
			return err
		}
		if count == 0 {
			return errors.Join(ErrInvalidInput, errors.New("destination is unavailable"))
		}
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
