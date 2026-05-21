package campaigns

import (
	"context"

	"github.com/devflex/traffoflex/apps/api-service/internal/mongostore"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoRepository struct {
	store *mongostore.Store[models.Campaign]
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		store: mongostore.New[models.Campaign](
			db.Collection("campaigns"),
			ErrInvalidInput,
			ErrNotFound,
		),
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
	return r.store.Update(
		ctx,
		campaign.ID,
		campaign,
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
