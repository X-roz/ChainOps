package db

import (
	"context"
	"log/slog"
)

var walletLog = slog.With("db", "[wallets]")

// GetIndexedAddressToMonitor returns all wallet addresses being actively
// watched on the given network (matched by network_key, case-insensitive).
func GetIndexedAddressToMonitor(ctx context.Context, networkKey string) ([]string, error) {
	rows, err := pool.Query(ctx,
		`SELECT iw.wallet_address
		 FROM indexed_wallets iw
		 JOIN networks n ON n.id = iw.network_id
		 WHERE iw.active_subscriber_count > 0
		   AND n.network_key = $1`,
		networkKey,
	)
	if err != nil {
		walletLog.Error("failed to query indexed wallets", "network", networkKey, "error", err)
		return nil, err
	}
	defer rows.Close()

	var addresses []string
	for rows.Next() {
		var addr string
		if err := rows.Scan(&addr); err != nil {
			walletLog.Error("failed to scan wallet address", "error", err)
			return nil, err
		}
		addresses = append(addresses, addr)
	}

	if err := rows.Err(); err != nil {
		walletLog.Error("error iterating indexed wallets", "error", err)
		return nil, err
	}

	return addresses, nil
}
