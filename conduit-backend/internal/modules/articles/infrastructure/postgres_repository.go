package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"conduit/backend/internal/modules/articles/domain"
	sharedomain "conduit/backend/internal/shared/domain"
	"conduit/backend/internal/shared/infrastructure/postgres"
)

const slugConstraint = "articles_slug_unique"

const tagsSQL = `COALESCE(
  (SELECT array_agg(t.tag ORDER BY t.position)
     FROM article_tags t WHERE t.article_id = a.id),
  ARRAY[]::varchar[]
)`

const viewColumnsTemplate = `
  a.id::text, a.author_id::text, a.slug, a.title, a.description, a.created_at, a.updated_at,
  ` + tagsSQL + ` AS tag_list,
  (SELECT count(*)::int FROM favorites f WHERE f.article_id = a.id) AS favorites_count,
  EXISTS (
    SELECT 1 FROM favorites f
     WHERE f.article_id = a.id AND f.user_id = %s::uuid
  ) AS favorited`

type PostgresRepository struct {
	db postgres.DB
}

func NewPostgresRepository(db postgres.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) NextID() string {
	return sharedomain.NewEntityID()
}

func (r *PostgresRepository) Add(ctx context.Context, article domain.Article) error {
	snapshot := article.Snapshot()
	err := postgres.WithTransaction(ctx, r.db, func(tx postgres.Querier) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO articles (id, author_id, slug, title, description, body, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			snapshot.ID, snapshot.AuthorID, snapshot.Slug, snapshot.Title, snapshot.Description,
			snapshot.Body, snapshot.CreatedAt, snapshot.UpdatedAt,
		)
		if err != nil {
			return err
		}
		return insertTags(ctx, tx, snapshot.ID, snapshot.TagList)
	})
	return translateConflict(err)
}

func (r *PostgresRepository) Update(ctx context.Context, article domain.Article) (bool, error) {
	snapshot := article.Snapshot()
	updated := false
	err := postgres.WithTransaction(ctx, r.db, func(tx postgres.Querier) error {
		tag, err := tx.Exec(ctx,
			`UPDATE articles
			    SET slug = $2, title = $3, description = $4, body = $5, updated_at = $6
			  WHERE id = $1`,
			snapshot.ID, snapshot.Slug, snapshot.Title, snapshot.Description, snapshot.Body, snapshot.UpdatedAt,
		)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return nil
		}
		if _, err := tx.Exec(ctx, "DELETE FROM article_tags WHERE article_id = $1", snapshot.ID); err != nil {
			return err
		}
		updated = true
		return insertTags(ctx, tx, snapshot.ID, snapshot.TagList)
	})
	if err != nil {
		return false, translateConflict(err)
	}
	return updated, nil
}

func (r *PostgresRepository) FindBySlug(ctx context.Context, slug string) (domain.Article, bool, error) {
	var snapshot domain.Snapshot
	err := r.db.QueryRow(ctx,
		`SELECT a.id::text, a.author_id::text, a.slug, a.title, a.description, a.body,
		        a.created_at, a.updated_at, `+tagsSQL+` AS tag_list
		   FROM articles a
		  WHERE a.slug = $1`,
		slug,
	).Scan(
		&snapshot.ID, &snapshot.AuthorID, &snapshot.Slug, &snapshot.Title, &snapshot.Description,
		&snapshot.Body, &snapshot.CreatedAt, &snapshot.UpdatedAt, &snapshot.TagList,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Article{}, false, nil
	}
	if err != nil {
		return domain.Article{}, false, err
	}
	return domain.Restore(snapshot), true, nil
}

func (r *PostgresRepository) FindViewBySlug(ctx context.Context, slug, viewerID string) (domain.View, bool, error) {
	var view domain.View
	err := r.db.QueryRow(ctx,
		"SELECT "+fmt.Sprintf(viewColumnsTemplate, "$2")+", a.body FROM articles a WHERE a.slug = $1",
		slug, nullable(viewerID),
	).Scan(
		&view.ID, &view.AuthorID, &view.Slug, &view.Title, &view.Description,
		&view.CreatedAt, &view.UpdatedAt, &view.TagList, &view.FavoritesCount, &view.Favorited, &view.Body,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.View{}, false, nil
	}
	if err != nil {
		return domain.View{}, false, err
	}
	return normalizeView(view), true, nil
}

func (r *PostgresRepository) FindPage(ctx context.Context, query domain.PageQuery) (domain.Page, error) {
	values := []any{}
	conditions := []string{}
	bind := func(value any) string {
		values = append(values, value)
		return fmt.Sprintf("$%d", len(values))
	}

	if query.Tag != nil {
		conditions = append(conditions,
			"EXISTS (SELECT 1 FROM article_tags t WHERE t.article_id = a.id AND t.tag = "+bind(*query.Tag)+")")
	}
	if query.AuthorID != "" {
		conditions = append(conditions, "a.author_id = "+bind(query.AuthorID)+"::uuid")
	}
	if query.AuthorIDs != nil {
		conditions = append(conditions, "a.author_id = ANY("+bind(query.AuthorIDs)+"::text[]::uuid[])")
	}
	if query.FavoritedByID != "" {
		conditions = append(conditions,
			"EXISTS (SELECT 1 FROM favorites f WHERE f.article_id = a.id AND f.user_id = "+bind(query.FavoritedByID)+"::uuid)")
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*)::int FROM articles a "+where, values...).Scan(&total); err != nil {
		return domain.Page{}, err
	}

	viewerRef := bind(nullable(query.ViewerID))
	limitRef := bind(query.Limit)
	offsetRef := bind(query.Offset)
	rows, err := r.db.Query(ctx,
		"SELECT "+fmt.Sprintf(viewColumnsTemplate, viewerRef)+" FROM articles a "+where+
			" ORDER BY a.created_at DESC, a.seq DESC LIMIT "+limitRef+" OFFSET "+offsetRef,
		values...,
	)
	if err != nil {
		return domain.Page{}, err
	}
	defer rows.Close()

	items := []domain.View{}
	for rows.Next() {
		var view domain.View
		if err := rows.Scan(
			&view.ID, &view.AuthorID, &view.Slug, &view.Title, &view.Description,
			&view.CreatedAt, &view.UpdatedAt, &view.TagList, &view.FavoritesCount, &view.Favorited,
		); err != nil {
			return domain.Page{}, err
		}
		items = append(items, normalizeView(view))
	}
	if err := rows.Err(); err != nil {
		return domain.Page{}, err
	}
	return domain.Page{Items: items, Total: total}, nil
}

func (r *PostgresRepository) SlugTaken(ctx context.Context, slug, excludeID string) (bool, error) {
	var taken bool
	err := r.db.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM articles WHERE slug = $1 AND ($2::text = '' OR id <> $2::text::uuid))",
		slug, excludeID,
	).Scan(&taken)
	return taken, err
}

func (r *PostgresRepository) DeleteByID(ctx context.Context, id string) (bool, error) {
	tag, err := r.db.Exec(ctx, "DELETE FROM articles WHERE id = $1", id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *PostgresRepository) Favorite(ctx context.Context, articleID, userID string) error {
	_, err := r.db.Exec(ctx,
		"INSERT INTO favorites (user_id, article_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		userID, articleID,
	)
	return err
}

func (r *PostgresRepository) Unfavorite(ctx context.Context, articleID, userID string) error {
	_, err := r.db.Exec(ctx,
		"DELETE FROM favorites WHERE user_id = $1 AND article_id = $2",
		userID, articleID,
	)
	return err
}

func (r *PostgresRepository) PopularTags(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.db.Query(ctx,
		`SELECT tag FROM article_tags
		  GROUP BY tag
		  ORDER BY count(*) DESC, tag COLLATE "C" ASC
		  LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := []string{}
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func insertTags(ctx context.Context, tx postgres.Querier, articleID string, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO article_tags (article_id, position, tag)
		 SELECT $1, ordinality - 1, tag
		   FROM unnest($2::text[]) WITH ORDINALITY AS t(tag, ordinality)`,
		articleID, tags,
	)
	return err
}

func normalizeView(view domain.View) domain.View {
	view.CreatedAt = view.CreatedAt.UTC()
	view.UpdatedAt = view.UpdatedAt.UTC()
	if view.TagList == nil {
		view.TagList = []string{}
	}
	return view
}

func translateConflict(err error) error {
	if err != nil && postgres.ConstraintName(err) == slugConstraint {
		return domain.ErrSlugTaken
	}
	return err
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
