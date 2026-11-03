package infrastructure

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"conduit/backend/internal/modules/comments/domain"
	"conduit/backend/internal/shared/infrastructure/mongodb"
)

type commentDocument struct {
	ID        int64     `bson:"_id"`
	ArticleID string    `bson:"articleId"`
	AuthorID  string    `bson:"authorId"`
	Body      string    `bson:"body"`
	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
}

func (d commentDocument) toComment() domain.Comment {
	return domain.Restore(domain.Snapshot{
		ID:        d.ID,
		ArticleID: d.ArticleID,
		AuthorID:  d.AuthorID,
		Body:      d.Body,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	})
}

type MongoRepository struct {
	db       *mongo.Database
	comments *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{db: db, comments: db.Collection(mongodb.CollectionComments)}
}

func (r *MongoRepository) Add(ctx context.Context, comment domain.Comment) (domain.Comment, error) {
	snapshot := comment.Snapshot()
	id, err := mongodb.NextSequence(ctx, r.db, mongodb.CounterComments)
	if err != nil {
		return domain.Comment{}, err
	}
	document := commentDocument{
		ID:        id,
		ArticleID: snapshot.ArticleID,
		AuthorID:  snapshot.AuthorID,
		Body:      snapshot.Body,
		CreatedAt: snapshot.CreatedAt,
		UpdatedAt: snapshot.UpdatedAt,
	}
	if _, err := r.comments.InsertOne(ctx, document); err != nil {
		return domain.Comment{}, err
	}
	return document.toComment(), nil
}

func (r *MongoRepository) FindByID(ctx context.Context, id int64) (domain.Comment, bool, error) {
	document, found, err := mongodb.FindOne[commentDocument](ctx, r.comments, bson.D{{Key: "_id", Value: id}})
	if err != nil || !found {
		return domain.Comment{}, false, err
	}
	return document.toComment(), true, nil
}

func (r *MongoRepository) ListByArticle(ctx context.Context, articleID string) ([]domain.Comment, error) {
	documents, err := mongodb.FindAll[commentDocument](ctx, r.comments,
		bson.D{{Key: "articleId", Value: articleID}},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}, {Key: "_id", Value: -1}}),
	)
	if err != nil {
		return nil, err
	}
	comments := make([]domain.Comment, 0, len(documents))
	for _, document := range documents {
		comments = append(comments, document.toComment())
	}
	return comments, nil
}

func (r *MongoRepository) DeleteByID(ctx context.Context, id int64) (bool, error) {
	result, err := r.comments.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return false, err
	}
	return result.DeletedCount == 1, nil
}

func (r *MongoRepository) DeleteByArticle(ctx context.Context, articleID string) (int64, error) {
	result, err := r.comments.DeleteMany(ctx, bson.D{{Key: "articleId", Value: articleID}})
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}
