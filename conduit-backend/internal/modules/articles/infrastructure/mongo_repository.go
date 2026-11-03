package infrastructure

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"conduit/backend/internal/modules/articles/domain"
	sharedomain "conduit/backend/internal/shared/domain"
	"conduit/backend/internal/shared/infrastructure/mongodb"
)

const slugIndex = "articles_slug_unique"

type articleDocument struct {
	ID          string    `bson:"_id"`
	AuthorID    string    `bson:"authorId"`
	Slug        string    `bson:"slug"`
	Title       string    `bson:"title"`
	Description string    `bson:"description"`
	Body        string    `bson:"body"`
	TagList     []string  `bson:"tagList"`
	FavoritedBy []string  `bson:"favoritedBy"`
	Seq         int64     `bson:"seq"`
	CreatedAt   time.Time `bson:"createdAt"`
	UpdatedAt   time.Time `bson:"updatedAt"`
}

type viewDocument struct {
	ID             string    `bson:"_id"`
	AuthorID       string    `bson:"authorId"`
	Slug           string    `bson:"slug"`
	Title          string    `bson:"title"`
	Description    string    `bson:"description"`
	Body           string    `bson:"body"`
	TagList        []string  `bson:"tagList"`
	CreatedAt      time.Time `bson:"createdAt"`
	UpdatedAt      time.Time `bson:"updatedAt"`
	FavoritesCount int       `bson:"favoritesCount"`
	Favorited      bool      `bson:"favorited"`
}

func (d viewDocument) toView() domain.View {
	tags := d.TagList
	if tags == nil {
		tags = []string{}
	}
	return domain.View{
		ID:             d.ID,
		AuthorID:       d.AuthorID,
		Slug:           d.Slug,
		Title:          d.Title,
		Description:    d.Description,
		Body:           d.Body,
		TagList:        tags,
		CreatedAt:      d.CreatedAt.UTC(),
		UpdatedAt:      d.UpdatedAt.UTC(),
		FavoritesCount: d.FavoritesCount,
		Favorited:      d.Favorited,
	}
}

func (d articleDocument) toArticle() domain.Article {
	return domain.Restore(domain.Snapshot{
		ID:          d.ID,
		AuthorID:    d.AuthorID,
		Slug:        d.Slug,
		Title:       d.Title,
		Description: d.Description,
		Body:        d.Body,
		TagList:     d.TagList,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	})
}

type MongoRepository struct {
	db       *mongo.Database
	articles *mongo.Collection
}

func NewMongoRepository(db *mongo.Database) *MongoRepository {
	return &MongoRepository{db: db, articles: db.Collection(mongodb.CollectionArticles)}
}

func (r *MongoRepository) NextID() string {
	return sharedomain.NewEntityID()
}

func (r *MongoRepository) Add(ctx context.Context, article domain.Article) error {
	snapshot := article.Snapshot()
	seq, err := mongodb.NextSequence(ctx, r.db, mongodb.CounterArticles)
	if err != nil {
		return err
	}
	_, err = r.articles.InsertOne(ctx, articleDocument{
		ID:          snapshot.ID,
		AuthorID:    snapshot.AuthorID,
		Slug:        snapshot.Slug,
		Title:       snapshot.Title,
		Description: snapshot.Description,
		Body:        snapshot.Body,
		TagList:     tagsOrEmpty(snapshot.TagList),
		FavoritedBy: []string{},
		Seq:         seq,
		CreatedAt:   snapshot.CreatedAt,
		UpdatedAt:   snapshot.UpdatedAt,
	})
	return translateConflict(err)
}

func (r *MongoRepository) Update(ctx context.Context, article domain.Article) (bool, error) {
	snapshot := article.Snapshot()
	result, err := r.articles.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: snapshot.ID}},
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "slug", Value: snapshot.Slug},
			{Key: "title", Value: snapshot.Title},
			{Key: "description", Value: snapshot.Description},
			{Key: "body", Value: snapshot.Body},
			{Key: "tagList", Value: tagsOrEmpty(snapshot.TagList)},
			{Key: "updatedAt", Value: snapshot.UpdatedAt},
		}}},
	)
	if err != nil {
		return false, translateConflict(err)
	}
	return result.MatchedCount == 1, nil
}

func (r *MongoRepository) FindBySlug(ctx context.Context, slug string) (domain.Article, bool, error) {
	document, found, err := mongodb.FindOne[articleDocument](ctx, r.articles, bson.D{{Key: "slug", Value: slug}})
	if err != nil || !found {
		return domain.Article{}, false, err
	}
	return document.toArticle(), true, nil
}

func (r *MongoRepository) FindViewBySlug(ctx context.Context, slug, viewerID string) (domain.View, bool, error) {
	views, err := mongodb.AggregateAll[viewDocument](ctx, r.articles, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "slug", Value: slug}}}},
		{{Key: "$limit", Value: 1}},
		viewProjection(viewerID, true),
	})
	if err != nil || len(views) == 0 {
		return domain.View{}, false, err
	}
	return views[0].toView(), true, nil
}

func (r *MongoRepository) FindPage(ctx context.Context, query domain.PageQuery) (domain.Page, error) {
	filter := pageFilter(query)

	total, err := r.articles.CountDocuments(ctx, filter)
	if err != nil {
		return domain.Page{}, err
	}
	if query.Limit <= 0 {
		return domain.Page{Items: []domain.View{}, Total: int(total)}, nil
	}

	documents, err := mongodb.AggregateAll[viewDocument](ctx, r.articles, mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$sort", Value: bson.D{{Key: "createdAt", Value: -1}, {Key: "seq", Value: -1}}}},
		{{Key: "$skip", Value: int64(query.Offset)}},
		{{Key: "$limit", Value: int64(query.Limit)}},
		viewProjection(query.ViewerID, false),
	})
	if err != nil {
		return domain.Page{}, err
	}

	items := make([]domain.View, 0, len(documents))
	for _, document := range documents {
		items = append(items, document.toView())
	}
	return domain.Page{Items: items, Total: int(total)}, nil
}

func (r *MongoRepository) SlugTaken(ctx context.Context, slug, excludeID string) (bool, error) {
	filter := bson.D{{Key: "slug", Value: slug}}
	if excludeID != "" {
		filter = append(filter, bson.E{Key: "_id", Value: bson.D{{Key: "$ne", Value: excludeID}}})
	}
	return mongodb.Exists(ctx, r.articles, filter)
}

func (r *MongoRepository) DeleteByID(ctx context.Context, id string) (bool, error) {
	result, err := r.articles.DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		return false, err
	}
	return result.DeletedCount == 1, nil
}

func (r *MongoRepository) Favorite(ctx context.Context, articleID, userID string) error {
	_, err := r.articles.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: articleID}},
		bson.D{{Key: "$addToSet", Value: bson.D{{Key: "favoritedBy", Value: userID}}}},
	)
	return err
}

func (r *MongoRepository) Unfavorite(ctx context.Context, articleID, userID string) error {
	_, err := r.articles.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: articleID}},
		bson.D{{Key: "$pull", Value: bson.D{{Key: "favoritedBy", Value: userID}}}},
	)
	return err
}

func (r *MongoRepository) PopularTags(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		return []string{}, nil
	}
	rows, err := mongodb.AggregateAll[struct {
		Tag string `bson:"_id"`
	}](ctx, r.articles, mongo.Pipeline{
		{{Key: "$unwind", Value: "$tagList"}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$tagList"},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}, {Key: "_id", Value: 1}}}},
		{{Key: "$limit", Value: int64(limit)}},
	})
	if err != nil {
		return nil, err
	}
	tags := make([]string, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, row.Tag)
	}
	return tags, nil
}

func pageFilter(query domain.PageQuery) bson.D {
	filter := bson.D{}
	if query.Tag != nil {
		filter = append(filter, bson.E{Key: "tagList", Value: *query.Tag})
	}
	if query.AuthorID != "" {
		filter = append(filter, bson.E{Key: "authorId", Value: query.AuthorID})
	}
	if query.AuthorIDs != nil {
		filter = append(filter, bson.E{Key: "authorId", Value: bson.D{{Key: "$in", Value: query.AuthorIDs}}})
	}
	if query.FavoritedByID != "" {
		filter = append(filter, bson.E{Key: "favoritedBy", Value: query.FavoritedByID})
	}
	return filter
}

func viewProjection(viewerID string, withBody bool) bson.D {
	favorites := bson.D{{Key: "$ifNull", Value: bson.A{"$favoritedBy", bson.A{}}}}

	var favorited any = bson.D{{Key: "$literal", Value: false}}
	if viewerID != "" {
		favorited = bson.D{{Key: "$in", Value: bson.A{bson.D{{Key: "$literal", Value: viewerID}}, favorites}}}
	}

	fields := bson.D{
		{Key: "authorId", Value: 1},
		{Key: "slug", Value: 1},
		{Key: "title", Value: 1},
		{Key: "description", Value: 1},
		{Key: "tagList", Value: 1},
		{Key: "createdAt", Value: 1},
		{Key: "updatedAt", Value: 1},
		{Key: "favoritesCount", Value: bson.D{{Key: "$size", Value: favorites}}},
		{Key: "favorited", Value: favorited},
	}
	if withBody {
		fields = append(fields, bson.E{Key: "body", Value: 1})
	}
	return bson.D{{Key: "$project", Value: fields}}
}

func tagsOrEmpty(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}

func translateConflict(err error) error {
	if mongodb.DuplicateKeyIndex(err) == slugIndex {
		return domain.ErrSlugTaken
	}
	return err
}
