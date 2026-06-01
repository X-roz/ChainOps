package db

import (
	"context"
	"log/slog"
	"math/big"
)

var sessionLog = slog.With("db", "[sessions]")

func CreateListenerSession(ctx context.Context, networkId string, fromBlock int64) (string, error) {
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO network_listener_sessions (network_id, from_block)
		 VALUES ($1, $2)
		 RETURNING id`,
		networkId, fromBlock,
	).Scan(&id)
	if err != nil {
		sessionLog.Error("failed to create listener session", "networkId", networkId, "error", err)
		return "", err
	}
	sessionLog.Info("listener session created", "sessionId", id, "networkId", networkId, "fromBlock", fromBlock)
	return id, nil
}

func CloseListenerSession(ctx context.Context, sessionId string, lastBlock *big.Int) error {
	var toBlock *int64
	if lastBlock != nil {
		v := lastBlock.Int64()
		toBlock = &v
	}
	_, err := pool.Exec(ctx,
		`UPDATE network_listener_sessions
		 SET status = 'CLOSED', to_block = $2, completed_at = NOW()
		 WHERE id = $1`,
		sessionId, toBlock,
	)
	if err != nil {
		sessionLog.Error("failed to close listener session", "sessionId", sessionId, "error", err)
		return err
	}
	sessionLog.Info("listener session closed", "sessionId", sessionId, "toBlock", toBlock)
	return nil
}
