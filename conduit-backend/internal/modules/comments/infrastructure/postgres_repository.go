package infrastructure

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"conduit/backend/internal/modules/comments/domain"
	"conduit/backend/internal/shared/infrastructure/postgres"
)

const commentColumns = "id, article_id::text, author_id::text, body, created_at, updated_at"

type PostgresRepository struct {
	db postgres.Querier
}

func NewPostgresRepository(db postgres.Querier) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Add(ctx context.Context, comment domain.Comment) (domain.Comment, error) {
	snapshot := comment.Snapshot()
	return scanComment(r.db.QueryRow(ctx,
		`INSERT INTO comments (article_id, author_id, body, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+commentColumns,
		snapshot.ArticleID, snapshot.AuthorID, snapshot.Body, snapshot.CreatedAt, snapshot.UpdatedAt,
	))
}

func (r *PostgresRepository) FindByID(ctx context.Context, id int64) (domain.Comment, bool, error) {
	comment, err := scanComment(r.db.QueryRow(ctx, "SELECT "+commentColumns+" FROM comments WHERE id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Comment{}, false, nil
	}
	if err != nil {
		return domain.Comment{}, false, err
	}
	return comment, true, nil
}

func (r *PostgresRepository) ListByArticle(ctx context.Context, articleID string) ([]domain.Comment, error) {
	rows, err := r.db.Query(ctx,
		"SELECT "+commentColumns+" FROM comments WHERE article_id = $1 ORDER BY created_at DESC, id DESC",
		articleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := []domain.Comment{}
	for rows.Next() {
		comment, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func (r *PostgresRepository) DeleteByID(ctx context.Context, id int64) (bool, error) {
	tag, err := r.db.Exec(ctx, "DELETE FROM comments WHERE id = $1", id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *PostgresRepository) DeleteByArticle(ctx context.Context, articleID string) (int64, error) {
	tag, err := r.db.Exec(ctx, "DELETE FROM comments WHERE article_id = $1", articleID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func scanComment(row pgx.Row) (domain.Comment, error) {
	var snapshot domain.Snapshot
	err := row.Scan(
		&snapshot.ID, &snapshot.ArticleID, &snapshot.AuthorID, &snapshot.Body,
		&snapshot.CreatedAt, &snapshot.UpdatedAt,
	)
	if err != nil {
		return domain.Comment{}, err
	}
	return domain.Restore(snapshot), nil
}
