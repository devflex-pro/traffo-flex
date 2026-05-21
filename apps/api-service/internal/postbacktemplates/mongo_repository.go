package postbacktemplates

import (
	"context"

	"github.com/devflex/traffoflex/apps/api-service/internal/mongostore"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoRepository struct {
	store *mongostore.Store[models.PostbackTemplate]
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		store: mongostore.New[models.PostbackTemplate](
			db.Collection("postback_templates"),
			ErrInvalidInput,
			ErrNotFound,
		),
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
	return r.store.Update(
		ctx,
		template.ID,
		template,
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
