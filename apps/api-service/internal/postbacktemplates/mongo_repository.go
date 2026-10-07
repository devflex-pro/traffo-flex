package postbacktemplates

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/devflex/traffoflex/apps/api-service/internal/mongostore"
	"github.com/devflex/traffoflex/apps/api-service/internal/scope"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoRepository struct {
	store    *mongostore.Store[models.PostbackTemplate]
	networks *mongo.Collection
	db       *mongo.Database
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		store: mongostore.New[models.PostbackTemplate](
			db.Collection("postback_templates"),
			ErrInvalidInput,
			ErrNotFound,
		),
		networks: db.Collection("affiliate_networks"),
		db:       db,
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
	if err := r.validateTemplate(
		ctx,
		template,
	); err != nil {
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
	if err := r.validateTemplate(
		ctx,
		template,
	); err != nil {
		return models.PostbackTemplate{}, err
	}
	return r.store.Update(
		ctx,
		template.ID,
		template,
	)
}

func (r *MongoRepository) validateTemplate(
	ctx context.Context,
	template models.PostbackTemplate,
) error {
	if template.Direction != "outgoing" {
		return r.validateNetwork(
			ctx,
			template.NetworkID,
		)
	}
	for collection, id := range map[string]string{"traffic_sources": template.SourceID, "campaigns": template.CampaignID} {
		if id == "" {
			continue
		}
		count, err := r.db.Collection(collection).CountDocuments(
			ctx,
			bson.M{"id": id, "owner_id": scope.OwnerID(ctx)},
		)
		if err != nil {
			return err
		}
		if count != 1 {
			return errors.Join(
				ErrInvalidInput,
				errors.New("postback scope is unavailable"),
			)
		}
	}
	return nil
}

func (r *MongoRepository) DeliveryJobs(ctx context.Context) (
	[]DeliveryJob,
	error,
) {
	cursor, err := r.db.Collection("outbound_postbacks").Find(
		ctx,
		bson.M{"owner_id": scope.OwnerID(ctx)},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(100),
	)
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cursor.All(
		ctx,
		&docs,
	); err != nil {
		return nil, err
	}
	items := make(
		[]DeliveryJob,
		0,
		len(docs),
	)
	for _, doc := range docs {
		data, err := json.Marshal(doc)
		if err != nil {
			return nil, err
		}
		var row DeliveryJob
		if err := json.Unmarshal(
			data,
			&row,
		); err != nil {
			return nil, err
		}
		items = append(
			items,
			row,
		)
	}
	return items, nil
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
