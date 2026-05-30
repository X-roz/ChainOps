package db

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

var netLog = slog.With("db", "[networks]")

// GetNetworkIDByKey resolves a network key (e.g. "sepolia") to its UUID.
// Returns an error if the network doesn't exist or is inactive.
func GetNetworkIDByKey(ctx context.Context, networkKey string) (string, error) {
	var id string
	err := pool.QueryRow(ctx,
		"SELECT id FROM networks WHERE network_key = $1 AND is_active = true",
		networkKey,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		netLog.Error("network not found or inactive", "key", networkKey)
		return "", err
	}
	if err != nil {
		netLog.Error("failed to query network", "key", networkKey, "error", err)
		return "", err
	}
	return id, nil
}
