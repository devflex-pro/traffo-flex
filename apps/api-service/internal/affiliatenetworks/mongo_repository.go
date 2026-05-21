package affiliatenetworks

import (
	"context"

	"github.com/devflex/traffoflex/apps/api-service/internal/mongostore"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoRepository struct {
	store *mongostore.Store[models.AffiliateNetwork]
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		store: mongostore.New[models.AffiliateNetwork](
			db.Collection("affiliate_networks"),
			ErrInvalidInput,
			ErrNotFound,
		),
	}
}

func (r *MongoRepository) List(ctx context.Context) (
	[]models.AffiliateNetwork,
	error,
) {
	return r.store.List(ctx)
}

func (r *MongoRepository) Get(
	ctx context.Context,
	id string,
) (
	models.AffiliateNetwork,
	error,
) {
	return r.store.Get(
		ctx,
		id,
	)
}

func (r *MongoRepository) Create(
	ctx context.Context,
	network models.AffiliateNetwork,
) (
	models.AffiliateNetwork,
	error,
) {
	return r.store.Create(
		ctx,
		network.ID,
		network,
	)
}

func (r *MongoRepository) Update(
	ctx context.Context,
	network models.AffiliateNetwork,
) (
	models.AffiliateNetwork,
	error,
) {
	return r.store.Update(
		ctx,
		network.ID,
		network,
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
