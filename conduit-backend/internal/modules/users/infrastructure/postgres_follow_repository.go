package infrastructure

import (
	"context"

	"conduit/backend/internal/shared/infrastructure/postgres"
)

type PostgresFollowRepository struct {
	db postgres.Querier
}

func NewPostgresFollowRepository(db postgres.Querier) *PostgresFollowRepository {
	return &PostgresFollowRepository{db: db}
}

func (r *PostgresFollowRepository) Follow(ctx context.Context, followerID, followedID string) error {
	_, err := r.db.Exec(ctx,
		"INSERT INTO follows (follower_id, followed_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
		followerID, followedID,
	)
	return err
}

func (r *PostgresFollowRepository) Unfollow(ctx context.Context, followerID, followedID string) error {
	_, err := r.db.Exec(ctx,
		"DELETE FROM follows WHERE follower_id = $1 AND followed_id = $2",
		followerID, followedID,
	)
	return err
}

func (r *PostgresFollowRepository) IsFollowing(ctx context.Context, followerID, followedID string) (bool, error) {
	rows, err := r.db.Query(ctx,
		"SELECT 1 FROM follows WHERE follower_id = $1 AND followed_id = $2",
		followerID, followedID,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	found := rows.Next()
	return found, rows.Err()
}

func (r *PostgresFollowRepository) FindFollowedAmong(ctx context.Context, followerID string, candidateIDs []string) (map[string]bool, error) {
	rows, err := r.db.Query(ctx,
		"SELECT followed_id::text FROM follows WHERE follower_id = $1 AND followed_id = ANY($2::text[]::uuid[])",
		followerID, candidateIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	followed := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		followed[id] = true
	}
	return followed, rows.Err()
}

func (r *PostgresFollowRepository) ListFollowedIDs(ctx context.Context, followerID string) ([]string, error) {
	rows, err := r.db.Query(ctx,
		"SELECT followed_id::text FROM follows WHERE follower_id = $1",
		followerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
