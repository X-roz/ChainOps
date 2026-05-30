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
	LastScannedBlock      *int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// GetIndexedAddressToMonitor returns all actively watched wallets for the given
// network. networkKey is matched case-insensitively against networks.network_key.
func GetIndexedAddressToMonitor(ctx context.Context, networkKey string) ([]IndexedAddress, error) {
	rows, err := pool.Query(ctx,
		`SELECT iw.wallet_address, iw.network_id,
		        iw.active_subscriber_count,
		        iw.last_scanned_block, iw.created_at, iw.updated_at
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

	var addresses []IndexedAddress
	for rows.Next() {
		var ia IndexedAddress
		if err := rows.Scan(
			&ia.WalletAddress,
			&ia.NetworkID,
			&ia.ActiveSubscriberCount,
			&ia.LastScannedBlock,
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

	return addresses, nil
}
