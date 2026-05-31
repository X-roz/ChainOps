package db

import (
	"context"
	"log/slog"
	"time"
)

var walletLog = slog.With("db", "[wallets]")

type IndexedAddress struct {
	WalletAddress         string
	NetworkID             string
	ActiveSubscriberCount int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// GetIndexedAddressToMonitor returns all actively watched wallets for the given
// network. networkKey is matched case-insensitively against networks.network_key.
func GetIndexedAddressToMonitor(ctx context.Context, networkId string) ([]IndexedAddress, error) {
	rows, err := pool.Query(ctx,
		`SELECT wallet_address, network_id,
		        active_subscriber_count,
		        created_at, updated_at
		 FROM indexed_wallets
		 WHERE network_id = $1
		   AND active_subscriber_count > 0`,
		networkId,
	)
	if err != nil {
		walletLog.Error("failed to query indexed wallets", "network", networkId, "error", err)
		return nil, err
	}
	defer rows.Close()

	var addresses []IndexedAddress
	for rows.Next() {
		var ia IndexedAddress
		if err := rows.Scan(
			&ia.WalletAddress,
			&ia.NetworkID,
			&ia.ActiveSubscriberCount,
			&ia.CreatedAt,
			&ia.UpdatedAt,
		); err != nil {
			walletLog.Error("failed to scan indexed wallet", "error", err)
			return nil, err
		}
		addresses = append(addresses, ia)
	}

	if err := rows.Err(); err != nil {
		walletLog.Error("error iterating indexed wallets", "error", err)
		return nil, err
	}

	walletLog.Info("indexed wallets fetched", "networkId", networkId, "count", len(addresses))
	return addresses, nil
}
