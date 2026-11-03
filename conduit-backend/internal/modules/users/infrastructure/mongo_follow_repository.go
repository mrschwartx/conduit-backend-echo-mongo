package infrastructure

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"conduit/backend/internal/shared/infrastructure/mongodb"
)

type followDocument struct {
	FollowedID string `bson:"followedId"`
}

type MongoFollowRepository struct {
	follows *mongo.Collection
}

func NewMongoFollowRepository(db *mongo.Database) *MongoFollowRepository {
	return &MongoFollowRepository{follows: db.Collection(mongodb.CollectionFollows)}
}

func (r *MongoFollowRepository) Follow(ctx context.Context, followerID, followedID string) error {
	_, err := r.follows.UpdateOne(ctx,
		pair(followerID, followedID),
		bson.D{{Key: "$setOnInsert", Value: bson.D{{Key: "createdAt", Value: time.Now().UTC()}}}},
		options.UpdateOne().SetUpsert(true),
	)
	if mongodb.IsDuplicateKey(err) {
		return nil
	}
	return err
}

func (r *MongoFollowRepository) Unfollow(ctx context.Context, followerID, followedID string) error {
	_, err := r.follows.DeleteOne(ctx, pair(followerID, followedID))
	return err
}

func (r *MongoFollowRepository) IsFollowing(ctx context.Context, followerID, followedID string) (bool, error) {
	return mongodb.Exists(ctx, r.follows, pair(followerID, followedID))
}

func (r *MongoFollowRepository) FindFollowedAmong(ctx context.Context, followerID string, candidateIDs []string) (map[string]bool, error) {
	followed := map[string]bool{}
	if len(candidateIDs) == 0 {
		return followed, nil
	}
	documents, err := mongodb.FindAll[followDocument](ctx, r.follows, bson.D{
		{Key: "followerId", Value: followerID},
		{Key: "followedId", Value: bson.D{{Key: "$in", Value: candidateIDs}}},
	})
	if err != nil {
		return nil, err
	}
	for _, document := range documents {
		followed[document.FollowedID] = true
	}
	return followed, nil
}

func (r *MongoFollowRepository) ListFollowedIDs(ctx context.Context, followerID string) ([]string, error) {
	documents, err := mongodb.FindAll[followDocument](ctx, r.follows, bson.D{{Key: "followerId", Value: followerID}})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(documents))
	for _, document := range documents {
		ids = append(ids, document.FollowedID)
	}
	return ids, nil
}

func pair(followerID, followedID string) bson.D {
	return bson.D{
		{Key: "followerId", Value: followerID},
		{Key: "followedId", Value: followedID},
	}
}
