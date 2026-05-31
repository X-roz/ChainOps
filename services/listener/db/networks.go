package db

import (
	"context"
	"errors"
	"log/slog"
	"math/big"

	"github.com/jackc/pgx/v5"
)

var netLog = slog.With("db", "[networks]")

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
	netLog.Info("network resolved", "key", networkKey, "id", id)
	return id, nil
}

// GetLastScannedBlock returns the highest block number already processed for
// this network. Returns 0 for a network that has never been scanned.
func GetLastScannedBlock(ctx context.Context, networkId string) (*big.Int, error) {
	var block int64
	err := pool.QueryRow(ctx,
		"SELECT last_scanned_block FROM networks WHERE id = $1",
		networkId,
	).Scan(&block)
	if errors.Is(err, pgx.ErrNoRows) {
		netLog.Error("network not found", "networkId", networkId)
		return nil, err
	}
	if err != nil {
		netLog.Error("failed to query last scanned block", "networkId", networkId, "error", err)
		return nil, err
	}
	netLog.Info("last scanned block fetched", "networkId", networkId, "block", block)
	return new(big.Int).SetInt64(block), nil
}

func UpdateLastScannedBlock(ctx context.Context, networkId string, block *big.Int) error {
	_, err := pool.Exec(ctx,
		"UPDATE networks SET last_scanned_block = $2 WHERE id = $1",
		networkId, block.Int64(),
	)
	if err != nil {
		netLog.Error("failed to update last scanned block", "networkId", networkId, "error", err)
		return err
	}
	netLog.Info("last scanned block updated", "networkId", networkId, "block", block)
	return nil
}
