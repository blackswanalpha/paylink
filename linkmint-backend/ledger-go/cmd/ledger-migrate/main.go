// Command ledger-migrate applies the double-entry ledger schema (work16): it creates the shared
// `ledger` schema, the append-only ledger_entries table and its UPDATE/DELETE-reject trigger, then
// exits. No service owns the ledger schema, so this one-shot migrator (wired into docker-compose
// alongside the other infra one-shots) applies it. Idempotent — safe to run on every boot.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	ledger "github.com/paylink/ledger-go"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "ledger-migrate")
	slog.SetDefault(log)

	dsn := os.Getenv("LEDGER_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://paylink:paylink@localhost:5432/paylink?sslmode=disable"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Error("postgres connect failed", "err", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	if err := ledger.NewMigrator(pool).Migrate(ctx); err != nil {
		log.Error("migrations failed", "err", err.Error())
		os.Exit(1)
	}
	log.Info("ledger schema ready")
}
