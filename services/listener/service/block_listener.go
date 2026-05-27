package service

import (
	"context"
	"listener/providers"
	"log/slog"
	"math/big"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

var addressToMonitor = "0x32056651573c19C329c9619DAF25A72e0D8a48dC"

func ListenToBlocks(ctx context.Context, providerList *[]providers.RPCProvider, safeBlockBuffer int64) {
	// nil means first run; using nil instead of 0 avoids ambiguity with genesis block
	var lastBlock *big.Int

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutting down block listener")
			return
		case <-ticker.C:
			client, provider, ok := getHealthyClient(providerList)
			if !ok {
				slog.Warn("no healthy providers available, skipping tick")
				continue
			}

			header, err := client.HeaderByNumber(ctx, nil)
			if err != nil {
				slog.Error("failed to fetch latest block", "provider", provider.Url, "error", err)
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
				slog.Info("no new confirmed blocks", "lastBlock", lastBlock, "safeBlock", safeBlock)
				continue
			}

			slog.Info("processing block range", "from", from, "to", safeBlock)

			for blockNum := new(big.Int).Set(from); blockNum.Cmp(safeBlock) <= 0; blockNum.Add(blockNum, big.NewInt(1)) {
				block, err := client.BlockByNumber(ctx, new(big.Int).Set(blockNum))
				if err != nil {
					slog.Error("failed to fetch block", "block", blockNum, "provider", provider.Url, "error", err)
					handleProviderFailure(provider, err)
					break
				}
				printIncomingTxns(block.Number(), block.Transactions())
				printOutGoingTxns(block.Number(), block.Transactions())
				lastBlock = new(big.Int).Set(blockNum)
			}
			slog.Info("finished processing blocks", "lastBlock", lastBlock)
		}
	}
}

func printIncomingTxns(blockNum *big.Int, txns types.Transactions) {
	target := common.HexToAddress(addressToMonitor)
	for _, tx := range txns {
		if tx.To() != nil && *tx.To() == target {
			slog.Info("incoming txn",
				"block", blockNum,
				"tx", tx.Hash().Hex(),
				"to", tx.To().Hex(),
				"value", tx.Value().String(),
			)
		}
	}
}

func printOutGoingTxns(blockNum *big.Int, txns types.Transactions) {
	target := common.HexToAddress(addressToMonitor)
	for _, tx := range txns {
		signer := types.LatestSignerForChainID(tx.ChainId())
		sender, err := types.Sender(signer, tx)
		if err != nil {
			slog.Error("failed to recover sender", "block", blockNum, "tx", tx.Hash().Hex(), "error", err)
			continue
		}
		if sender == target {
			slog.Info("outgoing txn",
				"block", blockNum,
				"tx", tx.Hash().Hex(),
				"from", sender.Hex(),
				"value", tx.Value().String(),
			)
		}
	}
}

// getHealthyClient returns a pointer to the first healthy provider so mutations
// (FailureCount, Status) persist back to the original slice.
func getHealthyClient(providerList *[]providers.RPCProvider) (*ethclient.Client, *providers.RPCProvider, bool) {
	for i := range *providerList {
		p := &(*providerList)[i]
		if p.Status != providers.Unhealthy {
			return p.Client, p, true
		}
	}
	return nil, nil, false
}

func handleProviderFailure(provider *providers.RPCProvider, err error) {
	if _, ok := err.(net.Error); !ok {
		return
	}
	provider.FailureCount++
	if provider.FailureCount >= 3 {
		slog.Warn("provider marked unhealthy", "provider", provider.Url, "failureCount", provider.FailureCount)
		provider.Status = providers.Unhealthy
	}
}
