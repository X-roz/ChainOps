package db

import (
	"context"
	"fmt"
	_ "listener/logger"
	"log/slog"

	"listener/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

var dblog = slog.With("db", "[connection]")
var pool *pgxpool.Pool

func Connect(ctx context.Context, cfg config.DatabaseConfig) error {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.DBName,
		cfg.SSLMode,
		cfg.Timezone,
	)

	var err error
	pool, err = pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database unreachable: %w", err)
	}

	dblog.Info("connected to database", "host", cfg.Host, "dbname", cfg.DBName)
	return nil
}
