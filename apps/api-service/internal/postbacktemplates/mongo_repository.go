package postbacktemplates

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
	store    *mongostore.Store[models.PostbackTemplate]
	networks *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		store: mongostore.New[models.PostbackTemplate](
			db.Collection("postback_templates"),
			ErrInvalidInput,
			ErrNotFound,
		),
		networks: db.Collection("affiliate_networks"),
	}
}

func (r *MongoRepository) List(ctx context.Context) (
	[]models.PostbackTemplate,
	error,
) {
	return r.store.List(ctx)
}

func (r *MongoRepository) Get(
	ctx context.Context,
	id string,
) (
	models.PostbackTemplate,
	error,
) {
	return r.store.Get(
		ctx,
		id,
	)
}

func (r *MongoRepository) Create(
	ctx context.Context,
	template models.PostbackTemplate,
) (
	models.PostbackTemplate,
	error,
) {
	if err := r.validateNetwork(ctx, template.NetworkID); err != nil {
		return models.PostbackTemplate{}, err
	}
	return r.store.Create(
		ctx,
		template.ID,
		template,
	)
}

func (r *MongoRepository) Update(
	ctx context.Context,
	template models.PostbackTemplate,
) (
	models.PostbackTemplate,
	error,
) {
	if err := r.validateNetwork(ctx, template.NetworkID); err != nil {
		return models.PostbackTemplate{}, err
	}
	return r.store.Update(
		ctx,
		template.ID,
		template,
	)
}

func (r *MongoRepository) validateNetwork(
	ctx context.Context,
	networkID string,
) error {
	ownerID := scope.OwnerID(ctx)
	if ownerID == "" || networkID == "" {
		return nil
	}
	count, err := r.networks.CountDocuments(
		ctx,
		bson.M{"id": networkID, "owner_id": ownerID},
	)
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.Join(ErrInvalidInput, errors.New("affiliate network is unavailable"))
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
