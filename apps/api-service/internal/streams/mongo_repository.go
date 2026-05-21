package streams

import (
	"context"

	"github.com/devflex/traffoflex/apps/api-service/internal/mongostore"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoRepository struct {
	store *mongostore.Store[models.Stream]
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		store: mongostore.New[models.Stream](
			db.Collection("streams"),
			ErrInvalidInput,
			ErrNotFound,
		),
	}
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
	return r.store.Update(
		ctx,
		stream.ID,
		stream,
	)
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
