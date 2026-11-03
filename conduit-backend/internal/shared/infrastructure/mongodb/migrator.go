package mongodb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const namespaceExistsCode = 48

type Migration struct {
	Name  string
	Apply func(ctx context.Context, db *mongo.Database) error
}

func Migrations() []Migration {
	return []Migration{
		{Name: "0001_create_users", Apply: createUsers},
		{Name: "0002_create_follows", Apply: createFollows},
		{Name: "0003_create_articles", Apply: createArticles},
		{Name: "0004_create_comments", Apply: createComments},
	}
}

func RunMigrations(ctx context.Context, db *mongo.Database, migrations []Migration, logger *slog.Logger) ([]string, error) {
	records := db.Collection(CollectionMigrations)
	executed := []string{}

	for _, migration := range migrations {
		applied, err := Exists(ctx, records, bson.D{{Key: "_id", Value: migration.Name}})
		if err != nil {
			return executed, err
		}
		if applied {
			continue
		}
		if err := migration.Apply(ctx, db); err != nil {
			return executed, fmt.Errorf("migration %s failed: %w", migration.Name, err)
		}
		_, err = records.InsertOne(ctx, bson.D{
			{Key: "_id", Value: migration.Name},
			{Key: "appliedAt", Value: time.Now().UTC()},
		})
		if IsDuplicateKey(err) {
			continue
		}
		if err != nil {
			return executed, err
		}
		executed = append(executed, migration.Name)
		if logger != nil {
			logger.Info("Applied migration " + migration.Name)
		}
	}
	return executed, nil
}

func createUsers(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection(CollectionUsers).Indexes().CreateMany(ctx, []mongo.IndexModel{
		unique("users_email_unique", asc("emailLower")),
		unique("users_username_unique", asc("usernameLower")),
	})
	return err
}

func createFollows(ctx context.Context, db *mongo.Database) error {
	notSelfFollow := bson.D{{Key: "$expr", Value: bson.D{{Key: "$ne", Value: bson.A{"$followerId", "$followedId"}}}}}
	if err := ensureCollection(ctx, db, CollectionFollows, notSelfFollow); err != nil {
		return err
	}
	_, err := db.Collection(CollectionFollows).Indexes().CreateMany(ctx, []mongo.IndexModel{
		unique("follows_pair_unique", asc("followerId"), asc("followedId")),
		index("follows_followed", asc("followedId")),
	})
	return err
}

func createArticles(ctx context.Context, db *mongo.Database) error {
	notReservedSlug := bson.D{{Key: "slug", Value: bson.D{{Key: "$ne", Value: "feed"}}}}
	if err := ensureCollection(ctx, db, CollectionArticles, notReservedSlug); err != nil {
		return err
	}
	_, err := db.Collection(CollectionArticles).Indexes().CreateMany(ctx, []mongo.IndexModel{
		unique("articles_slug_unique", asc("slug")),
		index("articles_author", asc("authorId")),
		index("articles_order", desc("createdAt"), desc("seq")),
		index("articles_tags", asc("tagList")),
		index("articles_favorited_by", asc("favoritedBy")),
	})
	return err
}

func createComments(ctx context.Context, db *mongo.Database) error {
	_, err := db.Collection(CollectionComments).Indexes().CreateMany(ctx, []mongo.IndexModel{
		index("comments_article", asc("articleId"), desc("createdAt"), desc("_id")),
		index("comments_author", asc("authorId")),
	})
	return err
}

func ensureCollection(ctx context.Context, db *mongo.Database, name string, validator bson.D) error {
	err := db.CreateCollection(ctx, name, options.CreateCollection().SetValidator(validator))
	var commandErr mongo.CommandError
	if errors.As(err, &commandErr) && commandErr.Code == namespaceExistsCode {
		return db.RunCommand(ctx, bson.D{
			{Key: "collMod", Value: name},
			{Key: "validator", Value: validator},
		}).Err()
	}
	return err
}

func asc(field string) bson.E {
	return bson.E{Key: field, Value: 1}
}

func desc(field string) bson.E {
	return bson.E{Key: field, Value: -1}
}

func index(name string, keys ...bson.E) mongo.IndexModel {
	return mongo.IndexModel{Keys: bson.D(keys), Options: options.Index().SetName(name)}
}

func unique(name string, keys ...bson.E) mongo.IndexModel {
	return mongo.IndexModel{Keys: bson.D(keys), Options: options.Index().SetName(name).SetUnique(true)}
}
