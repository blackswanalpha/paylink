package ledger

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrator applies the numbered ledger migrations against a pool.
type Migrator struct {
	pool *pgxpool.Pool
}

// NewMigrator wraps a pool for migration. Used by the one-shot cmd/ledger-migrate and by tests.
func NewMigrator(pool *pgxpool.Pool) *Migrator { return &Migrator{pool: pool} }

// Migrate applies any pending numbered SQL migrations in lexical order, each in its own
// transaction, tracking applied versions in ledger.schema_migrations. Idempotent: already-applied
// migrations are skipped. Applied migrations are never edited — new behavior is a new file.
func (m *Migrator) Migrate(ctx context.Context) error {
	if _, err := m.pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS ledger`); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if _, err := m.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS ledger.schema_migrations (
		version    text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		var applied bool
		if err := m.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM ledger.schema_migrations WHERE version=$1)`, name).
			Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}

		body, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := m.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO ledger.schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}
