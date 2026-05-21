package trafficsources

import (
	"context"

	"github.com/devflex/traffoflex/apps/api-service/internal/mongostore"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoRepository struct {
	store *mongostore.Store[models.TrafficSource]
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		store: mongostore.New[models.TrafficSource](
			db.Collection("traffic_sources"),
			ErrInvalidInput,
			ErrNotFound,
		),
	}
}

func (r *MongoRepository) List(ctx context.Context) (
	[]models.TrafficSource,
	error,
) {
	return r.store.List(ctx)
}

func (r *MongoRepository) Get(
	ctx context.Context,
	id string,
) (
	models.TrafficSource,
	error,
) {
	return r.store.Get(
		ctx,
		id,
	)
}

func (r *MongoRepository) Create(
	ctx context.Context,
	source models.TrafficSource,
) (
	models.TrafficSource,
	error,
) {
	return r.store.Create(
		ctx,
		source.ID,
		source,
	)
}

func (r *MongoRepository) Update(
	ctx context.Context,
	source models.TrafficSource,
) (
	models.TrafficSource,
	error,
) {
	return r.store.Update(
		ctx,
		source.ID,
		source,
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
