package mongodb

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func Connect(
	ctx context.Context,
	uri string,
) (
	*mongo.Client,
	error,
) {
	connectCtx, cancel := context.WithTimeout(
		ctx,
		10*time.Second,
	)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(
		connectCtx,
		readpref.Primary(),
	); err != nil {
		if disconnectErr := client.Disconnect(connectCtx); disconnectErr != nil {
			return nil, errors.Join(
				err,
				disconnectErr,
			)
		}
		return nil, err
	}
	return client, nil
}

func Disconnect(
	ctx context.Context,
	client *mongo.Client,
) error {
	if client == nil {
		return nil
	}
	disconnectCtx, cancel := context.WithTimeout(
		ctx,
		10*time.Second,
	)
	defer cancel()
	return client.Disconnect(disconnectCtx)
}
