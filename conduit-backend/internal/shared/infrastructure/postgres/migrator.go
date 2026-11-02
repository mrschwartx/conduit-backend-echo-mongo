package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const advisoryLockKey = 727001

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

func EmbeddedMigrations() fs.FS {
	sub, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		panic(err)
	}
	return sub
}

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, source fs.FS, logger *slog.Logger) ([]string, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return nil, err
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", advisoryLockKey)
	}()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
		  name       TEXT PRIMARY KEY,
		  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return nil, err
	}

	applied, err := appliedMigrations(ctx, conn)
	if err != nil {
		return nil, err
	}

	files, err := listMigrationFiles(source)
	if err != nil {
		return nil, err
	}

	executed := []string{}
	for _, file := range files {
		if applied[file] {
			continue
		}
		if err := applyMigration(ctx, conn, source, file); err != nil {
			return executed, fmt.Errorf("migration %s failed: %w", file, err)
		}
		executed = append(executed, file)
		if logger != nil {
			logger.Info("Applied migration " + file)
		}
	}
	return executed, nil
}

func appliedMigrations(ctx context.Context, conn Querier) (map[string]bool, error) {
	rows, err := conn.Query(ctx, "SELECT name FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

func listMigrationFiles(source fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(source, ".")
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	slices.Sort(files)
	return files, nil
}

func applyMigration(ctx context.Context, conn Querier, source fs.FS, file string) error {
	script, err := fs.ReadFile(source, file)
	if err != nil {
		return err
	}

	starter, ok := conn.(interface {
		Begin(ctx context.Context) (pgx.Tx, error)
	})
	if !ok {
		return fmt.Errorf("connection does not support transactions")
	}
	tx, err := starter.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(context.WithoutCancel(ctx))
	}()

	if _, err := tx.Exec(ctx, string(script)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (name) VALUES ($1)", file); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
