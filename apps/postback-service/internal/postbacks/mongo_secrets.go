package postbacks

import (
	"context"
	"strings"

	"github.com/devflex/traffoflex/apps/postback-service/internal/normalize"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type SecretStore interface {
	Secrets(
		ctx context.Context,
		networkID string,
	) ([]string, error)
}

type MongoSecretStore struct {
	collection *mongo.Collection
}

func NewMongoSecretStore(db *mongo.Database) *MongoSecretStore {
	return &MongoSecretStore{collection: db.Collection("postback_templates")}
}

func (s *MongoSecretStore) Secrets(
	ctx context.Context,
	networkID string,
) ([]string, error) {
	cursor, err := s.collection.Find(
		ctx,
		bson.M{"network_id": networkID, "direction": bson.M{"$ne": "outgoing"}},
		options.Find().SetProjection(bson.M{"secret": 1}),
	)
	if err != nil {
		return nil, err
	}
	var docs []struct {
		Secret string `bson:"secret"`
	}
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	secrets := make([]string, 0, len(docs))
	for _, doc := range docs {
		if secret := strings.TrimSpace(doc.Secret); secret != "" {
			secrets = append(secrets, secret)
		}
	}
	return secrets, nil
}

func (s *MongoSecretStore) Credentials(
	ctx context.Context,
	networkID string,
) ([]normalize.Credential, error) {
	cursor, err := s.collection.Find(
		ctx,
		bson.M{"network_id": networkID, "direction": bson.M{"$ne": "outgoing"}},
		options.Find().SetProjection(bson.M{"secret": 1, "owner_id": 1}),
	)
	if err != nil {
		return nil, err
	}
	var docs []struct {
		Secret  string `bson:"secret"`
		OwnerID string `bson:"owner_id"`
	}
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	credentials := make([]normalize.Credential, 0, len(docs))
	for _, doc := range docs {
		if secret := strings.TrimSpace(doc.Secret); secret != "" {
			credentials = append(credentials, normalize.Credential{
				OwnerID: doc.OwnerID,
				Secret:  secret,
			})
		}
	}
	return credentials, nil
}
