package destinations

import (
	"context"

	"github.com/devflex/traffoflex/apps/api-service/internal/mongostore"
	"github.com/devflex/traffoflex/packages/go-shared/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoRepository struct {
	store        *mongostore.Store[models.Destination]
	healthStates *mongo.Collection
}

type destinationHealthState struct {
	ID      string              `bson:"_id"`
	Current models.HealthStatus `bson:"current"`
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		store: mongostore.New[models.Destination](
			db.Collection("destinations"),
			ErrInvalidInput,
			ErrNotFound,
		),
		healthStates: db.Collection("destination_health"),
	}
}

func (r *MongoRepository) List(ctx context.Context) (
	[]models.Destination,
	error,
) {
	items, err := r.store.List(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.overlayHealthStatuses(
		ctx,
		items,
	); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *MongoRepository) Get(
	ctx context.Context,
	id string,
) (
	models.Destination,
	error,
) {
	destination, err := r.store.Get(
		ctx,
		id,
	)
	if err != nil {
		return models.Destination{}, err
	}
	items := []models.Destination{destination}
	if err := r.overlayHealthStatuses(
		ctx,
		items,
	); err != nil {
		return models.Destination{}, err
	}
	return items[0], nil
}

func (r *MongoRepository) overlayHealthStatuses(
	ctx context.Context,
	destinations []models.Destination,
) error {
	if len(destinations) == 0 {
		return nil
	}
	ids := make(bson.A, 0, len(destinations))
	for _, destination := range destinations {
		ids = append(ids, destination.ID)
	}
	cursor, err := r.healthStates.Find(
		ctx,
		bson.M{"_id": bson.M{"$in": ids}},
	)
	if err != nil {
		return err
	}
	var states []destinationHealthState
	if err := cursor.All(
		ctx,
		&states,
	); err != nil {
		return err
	}
	statusByID := make(map[string]models.HealthStatus, len(states))
	for _, state := range states {
		if state.Current.Valid() {
			statusByID[state.ID] = state.Current
		}
	}
	for index, destination := range destinations {
		if status, ok := statusByID[destination.ID]; ok {
			destinations[index].HealthStatus = status
		}
	}
	return nil
}

func (r *MongoRepository) Create(
	ctx context.Context,
	destination models.Destination,
) (
	models.Destination,
	error,
) {
	return r.store.Create(
		ctx,
		destination.ID,
		destination,
	)
}

func (r *MongoRepository) Update(
	ctx context.Context,
	destination models.Destination,
) (
	models.Destination,
	error,
) {
	return r.store.Update(
		ctx,
		destination.ID,
		destination,
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
