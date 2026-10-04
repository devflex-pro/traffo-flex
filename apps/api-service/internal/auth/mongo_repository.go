package auth

import (
	"context"
	"encoding/json"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoRepository struct {
	collection *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		collection: db.Collection("users"),
	}
}

func (r *MongoRepository) ListUsers(ctx context.Context) (
	[]User,
	error,
) {
	cursor, err := r.collection.Find(
		ctx,
		bson.M{},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	users := []User{}
	for cursor.Next(ctx) {
		user, err := decodeUserCursor(cursor)
		if err != nil {
			return nil, err
		}
		users = append(
			users,
			user,
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
	return users, nil
}

func (r *MongoRepository) GetUser(
	ctx context.Context,
	id string,
) (
	User,
	error,
) {
	return r.findOne(
		ctx,
		bson.M{"id": id},
	)
}

func (r *MongoRepository) GetUserByEmail(
	ctx context.Context,
	email string,
) (
	User,
	error,
) {
	return r.findOne(
		ctx,
		bson.M{"email": normalizeEmail(email)},
	)
}

func (r *MongoRepository) SaveUser(
	ctx context.Context,
	user User,
) (
	User,
	error,
) {
	doc, err := encodeUser(user)
	if err != nil {
		return User{}, err
	}
	doc["id"] = user.ID
	_, err = r.collection.UpdateOne(
		ctx,
		bson.M{"id": user.ID},
		bson.M{
			"$set": doc,
			"$setOnInsert": bson.M{
				"_id": user.ID,
			},
		},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return User{}, errors.Join(
				ErrInvalidInput,
				errors.New("email already exists"),
			)
		}
		return User{}, err
	}
	return user, nil
}

func (r *MongoRepository) RecordOTPFailure(
	ctx context.Context,
	id,
	otpHash string,
) error {
	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{
			"id":       id,
			"otp_hash": otpHash,
			"$expr": bson.M{"$lt": bson.A{
				bson.M{"$ifNull": bson.A{"$otp_failed_attempts", 0}},
				maxOTPAttempts,
			}},
		},
		bson.M{"$inc": bson.M{"otp_failed_attempts": 1}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrOTPAttempts
	}
	return nil
}

func (r *MongoRepository) RevokeSessions(
	ctx context.Context,
	id string,
	version int64,
) error {
	filter := bson.M{"id": id, "session_version": version}
	if version == 0 {
		filter["$or"] = bson.A{
			bson.M{"session_version": bson.M{"$exists": false}},
			bson.M{"session_version": 0},
		}
		delete(filter, "session_version")
	}
	result, err := r.collection.UpdateOne(
		ctx,
		filter,
		bson.M{"$inc": bson.M{"session_version": 1}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrInvalidToken
	}
	return nil
}

func (r *MongoRepository) findOne(
	ctx context.Context,
	filter bson.M,
) (
	User,
	error,
) {
	res := r.collection.FindOne(
		ctx,
		filter,
	)
	if err := res.Err(); err != nil {
		if errors.Is(
			err,
			mongo.ErrNoDocuments,
		) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}

	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return User{}, err
	}
	return decodeUser(raw)
}

func decodeUserCursor(cursor *mongo.Cursor) (
	User,
	error,
) {
	var raw bson.M
	if err := cursor.Decode(&raw); err != nil {
		return User{}, err
	}
	return decodeUser(raw)
}

func encodeUser(user User) (
	bson.M,
	error,
) {
	payload, err := json.Marshal(user)
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
	doc["otp_hash"] = user.OTPHash
	doc["otp_expires_at"] = user.OTPExpiresAt
	doc["otp_requested_at"] = user.OTPRequestedAt
	doc["otp_failed_attempts"] = user.OTPFailedAttempts
	doc["session_version"] = user.SessionVersion
	return doc, nil
}

func decodeUser(doc bson.M) (
	User,
	error,
) {
	delete(
		doc,
		"_id",
	)
	payload, err := json.Marshal(doc)
	if err != nil {
		return User{}, err
	}
	var user User
	if err := json.Unmarshal(
		payload,
		&user,
	); err != nil {
		return User{}, err
	}
	if value, ok := doc["otp_hash"].(string); ok {
		user.OTPHash = value
	}
	if value, ok := doc["otp_expires_at"].(bson.DateTime); ok {
		user.OTPExpiresAt = value.Time()
	}
	if value, ok := doc["otp_requested_at"].(bson.DateTime); ok {
		user.OTPRequestedAt = value.Time()
	}
	if value, ok := doc["otp_failed_attempts"].(int32); ok {
		user.OTPFailedAttempts = int(value)
	}
	if value, ok := doc["session_version"].(int64); ok {
		user.SessionVersion = value
	}
	return user, nil
}
