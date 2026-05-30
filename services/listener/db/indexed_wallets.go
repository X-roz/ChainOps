package db

import (
	"context"
	"log/slog"
)

func GetIndexedAddressToMonitor(ctx context.Context) []string {
	rows, err := pool.Query(ctx, "SELECT wallet_address FROM indexed_wallets WHERE active_subscriber_count > 0")
	if err != nil {
		slog.Error("failed to query indexed wallets", "error", err)
		return nil
	}
	defer rows.Close()

	var addresses []string
	for rows.Next() {
		var addr string
		if err := rows.Scan(&addr); err != nil {
			slog.Error("failed to scan wallet address", "error", err)
			return nil
		}
		addresses = append(addresses, addr)
	}

	if err := rows.Err(); err != nil {
		slog.Error("error iterating indexed wallets", "error", err)
		return nil
	}

	return addresses
}
