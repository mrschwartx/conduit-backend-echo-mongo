package infrastructure

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"conduit/backend/internal/modules/users/domain"
	sharedomain "conduit/backend/internal/shared/domain"
	"conduit/backend/internal/shared/infrastructure/postgres"
)

const userColumns = "id, username, email, password_hash, bio, image, created_at, updated_at"

var fieldByConstraint = map[string]string{
	"users_email_unique":    "email",
	"users_username_unique": "username",
}

type PostgresRepository struct {
	db postgres.Querier
}

func NewPostgresRepository(db postgres.Querier) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) NextID() string {
	return sharedomain.NewEntityID()
}

func (r *PostgresRepository) Add(ctx context.Context, user domain.User) error {
	snapshot := user.Snapshot()
	_, err := r.db.Exec(ctx,
		"INSERT INTO users ("+userColumns+") VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		snapshot.ID, snapshot.Username, snapshot.Email, snapshot.PasswordHash,
		snapshot.Bio, snapshot.Image, snapshot.CreatedAt, snapshot.UpdatedAt,
	)
	return translateConflict(err)
}

func (r *PostgresRepository) Update(ctx context.Context, user domain.User) (bool, error) {
	snapshot := user.Snapshot()
	tag, err := r.db.Exec(ctx,
		`UPDATE users
		    SET username = $2,
		        email = $3,
		        password_hash = $4,
		        bio = $5,
		        image = $6,
		        updated_at = $7
		  WHERE id = $1`,
		snapshot.ID, snapshot.Username, snapshot.Email, snapshot.PasswordHash,
		snapshot.Bio, snapshot.Image, snapshot.UpdatedAt,
	)
	if err != nil {
		return false, translateConflict(err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (domain.User, bool, error) {
	return r.findOne(ctx, "SELECT "+userColumns+" FROM users WHERE id = $1", id)
}

func (r *PostgresRepository) FindByEmail(ctx context.Context, email string) (domain.User, bool, error) {
	return r.findOne(ctx, "SELECT "+userColumns+" FROM users WHERE lower(email) = lower($1)", email)
}

func (r *PostgresRepository) FindByUsername(ctx context.Context, username string) (domain.User, bool, error) {
	return r.findOne(ctx, "SELECT "+userColumns+" FROM users WHERE lower(username) = lower($1)", username)
}

func (r *PostgresRepository) FindByIDs(ctx context.Context, ids []string) ([]domain.User, error) {
	rows, err := r.db.Query(ctx,
		"SELECT "+userColumns+" FROM users WHERE id = ANY($1::text[]::uuid[])",
		ids,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []domain.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *PostgresRepository) ExistsByID(ctx context.Context, id string) (bool, error) {
	var marker int
	err := r.db.QueryRow(ctx, "SELECT 1 FROM users WHERE id = $1", id).Scan(&marker)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *PostgresRepository) FindTakenFields(ctx context.Context, query domain.TakenQuery) ([]string, error) {
	var email, username bool
	err := r.db.QueryRow(ctx,
		`SELECT
		   COALESCE(bool_or(lower(email) = lower($1::text)), false),
		   COALESCE(bool_or(lower(username) = lower($2::text)), false)
		   FROM users
		  WHERE ($3::text = '' OR id <> $3::text::uuid)`,
		nullable(query.Email), nullable(query.Username), query.ExcludeID,
	).Scan(&email, &username)
	if err != nil {
		return nil, err
	}
	taken := []string{}
	if email {
		taken = append(taken, "email")
	}
	if username {
		taken = append(taken, "username")
	}
	return taken, nil
}

func (r *PostgresRepository) findOne(ctx context.Context, sql string, args ...any) (domain.User, bool, error) {
	user, err := scanUser(r.db.QueryRow(ctx, sql, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, false, nil
	}
	if err != nil {
		return domain.User{}, false, err
	}
	return user, true, nil
}

func scanUser(row pgx.Row) (domain.User, error) {
	var snapshot domain.Snapshot
	err := row.Scan(
		&snapshot.ID, &snapshot.Username, &snapshot.Email, &snapshot.PasswordHash,
		&snapshot.Bio, &snapshot.Image, &snapshot.CreatedAt, &snapshot.UpdatedAt,
	)
	if err != nil {
		return domain.User{}, err
	}
	return domain.Restore(snapshot), nil
}

func translateConflict(err error) error {
	if err == nil {
		return nil
	}
	if field, ok := fieldByConstraint[postgres.ConstraintName(err)]; ok {
		return sharedomain.NewConflictError(field)
	}
	return err
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
