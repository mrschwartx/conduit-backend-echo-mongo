package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const sequenceAttempts = 3

func NextSequence(ctx context.Context, db *mongo.Database, name string) (int64, error) {
	filter := bson.D{{Key: "_id", Value: name}}
	update := bson.D{{Key: "$inc", Value: bson.D{{Key: "seq", Value: int64(1)}}}}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var counter struct {
		Seq int64 `bson:"seq"`
	}
	var err error
	for range sequenceAttempts {
		err = db.Collection(CollectionCounters).FindOneAndUpdate(ctx, filter, update, opts).Decode(&counter)
		if err == nil {
			return counter.Seq, nil
		}
		if !mongo.IsDuplicateKeyError(err) {
			break
		}
	}
	return 0, err
}
