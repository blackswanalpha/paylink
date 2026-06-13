package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	ledger "github.com/paylink/ledger-go"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies the settlement-schema migrations and then the shared ledger-schema migrations
// (work16). Each numbered SQL file runs in its own transaction, tracked in
// settlement.schema_migrations. Idempotent: already-applied migrations are skipped; applied
// migrations are never edited — new behavior is a new file.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS settlement`); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if _, err := s.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS settlement.schema_migrations (
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
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM settlement.schema_migrations WHERE version=$1)`, name).
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

		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO settlement.schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	// The shared ledger schema (work16). Settlement posts balanced entries on its own transactions
	// via ledger.Post; the tables must exist. Idempotent — safe to run on every boot.
	if err := ledger.NewMigrator(s.pool).Migrate(ctx); err != nil {
		return fmt.Errorf("ledger migrate: %w", err)
	}
	return nil
}
