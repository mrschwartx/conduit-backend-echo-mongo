package mongodb

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func FindOne[T any](ctx context.Context, collection *mongo.Collection, filter any, opts ...options.Lister[options.FindOneOptions]) (T, bool, error) {
	var document T
	err := collection.FindOne(ctx, filter, opts...).Decode(&document)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return document, false, nil
	}
	if err != nil {
		return document, false, err
	}
	return document, true, nil
}

func FindAll[T any](ctx context.Context, collection *mongo.Collection, filter any, opts ...options.Lister[options.FindOptions]) ([]T, error) {
	cursor, err := collection.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	return drain[T](ctx, cursor)
}

func AggregateAll[T any](ctx context.Context, collection *mongo.Collection, pipeline any) ([]T, error) {
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	return drain[T](ctx, cursor)
}

func Exists(ctx context.Context, collection *mongo.Collection, filter any) (bool, error) {
	count, err := collection.CountDocuments(ctx, filter, options.Count().SetLimit(1))
	return count > 0, err
}

func drain[T any](ctx context.Context, cursor *mongo.Cursor) ([]T, error) {
	documents := []T{}
	if err := cursor.All(ctx, &documents); err != nil {
		return nil, err
	}
	return documents, nil
}
