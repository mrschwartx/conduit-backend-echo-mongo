package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type DB interface {
	Querier
	Begin(ctx context.Context) (pgx.Tx, error)
}

func WithTransaction(ctx context.Context, db DB, work func(Querier) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	if err := work(tx); err != nil {
		_ = tx.Rollback(context.WithoutCancel(ctx))
		return err
	}
	return tx.Commit(ctx)
}
