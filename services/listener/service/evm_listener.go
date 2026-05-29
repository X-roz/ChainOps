package service

import (
	"context"
	"errors"
	"listener/providers"
	"log/slog"
	"math/big"
	"net"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

var evmLog = slog.With("listener", "[evm_listener]")

var addressToMonitor = common.HexToAddress("0x32056651573c19C329c9619DAF25A72e0D8a48dC")

// httpStatusErrors are substrings found in RPC error messages that indicate
// server-side or rate-limit failures worth counting against a provider.
var httpStatusErrors = []string{"429", "500", "502", "503", "504"}

func EvmListener(ctx context.Context, providerList []*providers.EVMProvider, safeBlockBuffer int64) {

	var lastBlock *big.Int
	signer := types.LatestSignerForChainID(providerList[0].ChainID())
	nextProviderRetry := time.Now().Add(5 * time.Minute)
	var shouldRetry bool

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			evmLog.Info("shutting down block listener")
			return
		case <-ticker.C:

			shouldRetry, nextProviderRetry = requireRetry(nextProviderRetry)
			if shouldRetry {
				recoverUnhealthyProviders(ctx, providerList)
			}

			client, provider, ok := getHealthyClient(providerList)
			if !ok {
				evmLog.Warn("no healthy providers available, skipping tick")
				continue
			}

			header, err := client.HeaderByNumber(ctx, nil)
			if err != nil {
				evmLog.Error("failed to fetch latest block", "provider", provider.URL(), "error", err)
				handleProviderFailure(provider, err)
				continue
			}

			safeBlock := new(big.Int).Sub(header.Number, big.NewInt(safeBlockBuffer))

			var from *big.Int
			if lastBlock == nil {
				from = safeBlock
			} else {
				from = new(big.Int).Add(lastBlock, big.NewInt(1))
			}

			if from.Cmp(safeBlock) > 0 {
				evmLog.Info("no new confirmed blocks", "lastBlock", lastBlock, "safeBlock", safeBlock)
				continue
			}

			evmLog.Info("processing block range", "from", from, "to", safeBlock)

			for blockNum := new(big.Int).Set(from); blockNum.Cmp(safeBlock) <= 0; blockNum.Add(blockNum, big.NewInt(1)) {
				block, err := client.BlockByNumber(ctx, new(big.Int).Set(blockNum))
				if err != nil {
					evmLog.Error("failed to fetch block", "block", blockNum, "provider", provider.URL(), "error", err)
					handleProviderFailure(provider, err)
					break
				}
				printIncomingTxns(block.Number(), block.Transactions())
				printOutGoingTxns(block.Number(), signer, block.Transactions())
				lastBlock = new(big.Int).Set(blockNum)
			}
			evmLog.Info("finished processing blocks", "lastBlock", lastBlock)
		}
	}
}

func printIncomingTxns(blockNum *big.Int, txns types.Transactions) {
	for _, tx := range txns {
		if tx.To() != nil && *tx.To() == addressToMonitor {
			evmLog.Info("incoming txn",
				"block", blockNum,
				"tx", tx.Hash().Hex(),
				"to", tx.To().Hex(),
				"value", tx.Value().String(),
			)
		}
	}
}

func printOutGoingTxns(blockNum *big.Int, signer types.Signer, txns types.Transactions) {
	for _, tx := range txns {
		sender, err := types.Sender(signer, tx)
		if err != nil {
			evmLog.Error("failed to recover sender", "block", blockNum, "tx", tx.Hash().Hex(), "error", err)
			continue
		}
		if sender == addressToMonitor {
			evmLog.Info("outgoing txn",
				"block", blockNum,
				"tx", tx.Hash().Hex(),
				"from", sender.Hex(),
				"value", tx.Value().String(),
			)
		}
	}
}

func getHealthyClient(providerList []*providers.EVMProvider) (*ethclient.Client, *providers.EVMProvider, bool) {
	for _, p := range providerList {
		if p.IsHealthy() {
			return p.Client(), p, true
		}
	}
	return nil, nil, false
}

func isTransientProviderError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if _, ok := err.(net.Error); ok {
		return true
	}
	msg := err.Error()
	for _, code := range httpStatusErrors {
		if strings.Contains(msg, code) {
			return true
		}
	}
	return false
}

func handleProviderFailure(provider providers.Provider, err error) {
	if !isTransientProviderError(err) {
		return
	}
	provider.RecordFailure()
}

func requireRetry(nextRetry time.Time) (bool, time.Time) {
	if time.Now().Before(nextRetry) {
		return false, nextRetry
	}
	return true, time.Now().Add(5 * time.Minute)
}

func recoverUnhealthyProviders(ctx context.Context, providerList []*providers.EVMProvider) {
	for _, provider := range providerList {
		if !provider.IsHealthy() {
			provider.Recover(ctx)
		}
	}
}
