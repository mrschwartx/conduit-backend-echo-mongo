package infrastructure

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"conduit/backend/internal/modules/users/domain"
	sharedomain "conduit/backend/internal/shared/domain"
	"conduit/backend/internal/shared/infrastructure/mongodb"
)

var fieldByIndex = map[string]string{
	"users_email_unique":    "email",
	"users_username_unique": "username",
}

type userDocument struct {
	ID            string    `bson:"_id"`
	Username      string    `bson:"username"`
	UsernameLower string    `bson:"usernameLower"`
	Email         string    `bson:"email"`
	EmailLower    string    `bson:"emailLower"`
	PasswordHash  string    `bson:"passwordHash"`
	Bio           *string   `bson:"bio"`
	Image         *string   `bson:"image"`
	CreatedAt     time.Time `bson:"createdAt"`
	UpdatedAt     time.Time `bson:"updatedAt"`
}

func newUserDocument(snapshot domain.Snapshot) userDocument {
	return userDocument{
		ID:            snapshot.ID,
		Username:      snapshot.Username,
		UsernameLower: strings.ToLower(snapshot.Username),
		Email:         snapshot.Email,
		EmailLower:    strings.ToLower(snapshot.Email),
		PasswordHash:  snapshot.PasswordHash,
		Bio:           snapshot.Bio,
		Image:         snapshot.Image,
		CreatedAt:     snapshot.CreatedAt,
		UpdatedAt:     snapshot.UpdatedAt,
	}
}

func (d userDocument) toUser() domain.User {
	return domain.Restore(domain.Snapshot{
		ID:           d.ID,
		Username:     d.Username,
		Email:        d.Email,
		PasswordHash: d.PasswordHash,
		Bio:          d.Bio,
		Image:        d.Image,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	})
}

type MongoRepository struct {
	users *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{users: db.Collection(mongodb.CollectionUsers)}
}

func (r *MongoRepository) NextID() string {
	return sharedomain.NewEntityID()
}

func (r *MongoRepository) Add(ctx context.Context, user domain.User) error {
	_, err := r.users.InsertOne(ctx, newUserDocument(user.Snapshot()))
	return translateConflict(err)
}

func (r *MongoRepository) Update(ctx context.Context, user domain.User) (bool, error) {
	document := newUserDocument(user.Snapshot())
	result, err := r.users.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: document.ID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "username", Value: document.Username},
			{Key: "usernameLower", Value: document.UsernameLower},
			{Key: "email", Value: document.Email},
			{Key: "emailLower", Value: document.EmailLower},
			{Key: "passwordHash", Value: document.PasswordHash},
			{Key: "bio", Value: document.Bio},
			{Key: "image", Value: document.Image},
			{Key: "updatedAt", Value: document.UpdatedAt},
		}}},
	)
	if err != nil {
		return false, translateConflict(err)
	}
	return result.MatchedCount == 1, nil
}

func (r *MongoRepository) FindByID(ctx context.Context, id string) (domain.User, bool, error) {
	return r.findOne(ctx, bson.D{{Key: "_id", Value: id}})
}

func (r *MongoRepository) FindByEmail(ctx context.Context, email string) (domain.User, bool, error) {
	return r.findOne(ctx, bson.D{{Key: "emailLower", Value: strings.ToLower(email)}})
}

func (r *MongoRepository) FindByUsername(ctx context.Context, username string) (domain.User, bool, error) {
	return r.findOne(ctx, bson.D{{Key: "usernameLower", Value: strings.ToLower(username)}})
}

func (r *MongoRepository) FindByIDs(ctx context.Context, ids []string) ([]domain.User, error) {
	if len(ids) == 0 {
		return []domain.User{}, nil
	}
	documents, err := mongodb.FindAll[userDocument](ctx, r.users,
		bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}},
	)
	if err != nil {
		return nil, err
	}
	users := make([]domain.User, 0, len(documents))
	for _, document := range documents {
		users = append(users, document.toUser())
	}
	return users, nil
}

func (r *MongoRepository) ExistsByID(ctx context.Context, id string) (bool, error) {
	return mongodb.Exists(ctx, r.users, bson.D{{Key: "_id", Value: id}})
}

func (r *MongoRepository) FindTakenFields(ctx context.Context, query domain.TakenQuery) ([]string, error) {
	email := strings.ToLower(query.Email)
	username := strings.ToLower(query.Username)

	candidates := bson.A{}
	if email != "" {
		candidates = append(candidates, bson.D{{Key: "emailLower", Value: email}})
	}
	if username != "" {
		candidates = append(candidates, bson.D{{Key: "usernameLower", Value: username}})
	}
	if len(candidates) == 0 {
		return []string{}, nil
	}

	filter := bson.D{{Key: "$or", Value: candidates}}
	if query.ExcludeID != "" {
		filter = append(filter, bson.E{Key: "_id", Value: bson.D{{Key: "$ne", Value: query.ExcludeID}}})
	}

	matches, err := mongodb.FindAll[userDocument](ctx, r.users, filter)
	if err != nil {
		return nil, err
	}

	emailTaken, usernameTaken := false, false
	for _, match := range matches {
		emailTaken = emailTaken || (email != "" && match.EmailLower == email)
		usernameTaken = usernameTaken || (username != "" && match.UsernameLower == username)
	}

	taken := []string{}
	if emailTaken {
		taken = append(taken, "email")
	}
	if usernameTaken {
		taken = append(taken, "username")
	}
	return taken, nil
}

func (r *MongoRepository) findOne(ctx context.Context, filter bson.D) (domain.User, bool, error) {
	document, found, err := mongodb.FindOne[userDocument](ctx, r.users, filter)
	if err != nil || !found {
		return domain.User{}, false, err
	}
	return document.toUser(), true, nil
}

func translateConflict(err error) error {
	if err == nil {
		return nil
	}
	if field, ok := fieldByIndex[mongodb.DuplicateKeyIndex(err)]; ok {
		return sharedomain.NewConflictError(field)
	}
	return err
}
