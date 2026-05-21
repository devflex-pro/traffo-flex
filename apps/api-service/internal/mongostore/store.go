package mongostore

import (
	"context"
	"encoding/json"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Store[T any] struct {
	collection  *mongo.Collection
	invalidErr  error
	notFoundErr error
}

func New[T any](
	collection *mongo.Collection,
	invalidErr,
	notFoundErr error,
) *Store[T] {
	return &Store[T]{
		collection:  collection,
		invalidErr:  invalidErr,
		notFoundErr: notFoundErr,
	}
}

func (s *Store[T]) List(ctx context.Context) (
	[]T,
	error,
) {
	return s.list(
		ctx,
		bson.M{},
	)
}

func (s *Store[T]) ListByField(
	ctx context.Context,
	field,
	value string,
) (
	[]T,
	error,
) {
	return s.list(
		ctx,
		bson.M{field: value},
	)
}

func (s *Store[T]) Get(
	ctx context.Context,
	id string,
) (
	T,
	error,
) {
	var zero T
	res := s.collection.FindOne(
		ctx,
		bson.M{"id": id},
	)
	if err := res.Err(); err != nil {
		if errors.Is(
			err,
			mongo.ErrNoDocuments,
		) {
			return zero, s.notFoundErr
		}
		return zero, err
	}

	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return zero, err
	}
	item, err := decode[T](raw)
	if err != nil {
		return zero, err
	}
	return item, nil
}

func (s *Store[T]) Create(
	ctx context.Context,
	id string,
	item T,
) (
	T,
	error,
) {
	doc, err := encode(item)
	if err != nil {
		var zero T
		return zero, err
	}
	doc["_id"] = id
	doc["id"] = id

	if _, err := s.collection.InsertOne(
		ctx,
		doc,
	); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			var zero T
			return zero, errors.Join(
				s.invalidErr,
				errors.New("id already exists"),
			)
		}
		var zero T
		return zero, err
	}
	return item, nil
}

func (s *Store[T]) Update(
	ctx context.Context,
	id string,
	item T,
) (
	T,
	error,
) {
	doc, err := encode(item)
	if err != nil {
		var zero T
		return zero, err
	}
	doc["id"] = id
	delete(
		doc,
		"_id",
	)

	res, err := s.collection.UpdateOne(
		ctx,
		bson.M{"id": id},
		bson.M{"$set": doc},
	)
	if err != nil {
		var zero T
		return zero, err
	}
	if res.MatchedCount == 0 {
		var zero T
		return zero, s.notFoundErr
	}
	return item, nil
}

func (s *Store[T]) Delete(
	ctx context.Context,
	id string,
) error {
	res, err := s.collection.DeleteOne(
		ctx,
		bson.M{"id": id},
	)
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return s.notFoundErr
	}
	return nil
}

func (s *Store[T]) list(
	ctx context.Context,
	filter bson.M,
) (
	[]T,
	error,
) {
	cursor, err := s.collection.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	var items []T
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}
		item, err := decode[T](raw)
		if err != nil {
			return nil, err
		}
		items = append(
			items,
			item,
		)
	}
	if err := cursor.Err(); err != nil {
		if closeErr := cursor.Close(ctx); closeErr != nil {
			return nil, errors.Join(
				err,
				closeErr,
			)
		}
		return nil, err
	}
	if err := cursor.Close(ctx); err != nil {
		return nil, err
	}
	if items == nil {
		items = []T{}
	}
	return items, nil
}

func encode(item any) (
	bson.M,
	error,
) {
	payload, err := json.Marshal(item)
	if err != nil {
		return nil, err
	}
	var doc bson.M
	if err := json.Unmarshal(
		payload,
		&doc,
	); err != nil {
		return nil, err
	}
	return doc, nil
}

func decode[T any](doc bson.M) (
	T,
	error,
) {
	var item T
	delete(
		doc,
		"_id",
	)
	payload, err := json.Marshal(doc)
	if err != nil {
		return item, err
	}
	if err := json.Unmarshal(
		payload,
		&item,
	); err != nil {
		return item, err
	}
	return item, nil
}
