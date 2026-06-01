package service

import (
	"context"
	"errors"
	"fmt"
	"listener/db"
	_ "listener/logger"
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

// httpStatusErrors are substrings found in RPC error messages that indicate
// server-side or rate-limit failures worth counting against a provider.
var httpStatusErrors = []string{"429", "500", "502", "503", "504"}

type EvmListener struct {
	providerList     []*providers.EVMProvider
	safeBlockBuffer  int64
	maxBlocksPerTick int64
	usdcListen       bool
	networkId        string
}

func NewEvmListener(providerList []*providers.EVMProvider, safeBlockBuffer int64, maxBlocksPerTick int64, usdcListen bool, networkId string) *EvmListener {
	return &EvmListener{
		providerList:     providerList,
		safeBlockBuffer:  safeBlockBuffer,
		maxBlocksPerTick: maxBlocksPerTick,
		usdcListen:       usdcListen,
		networkId:        networkId,
	}
}

func (el *EvmListener) Run(ctx context.Context) {
	signer := types.LatestSignerForChainID(el.providerList[0].ChainID())
	nextProviderRetry := time.Now().Add(5 * time.Minute)
	var shouldRetry bool
	var lastProcessedBlock *big.Int

	// Create a session on startup using the current chain tip as the reference point.
	// A separate background context is not needed here — startup context is still live.
	var sessionId string
	if client, _, ok := getHealthyClient(el.providerList); ok {
		if header, err := client.HeaderByNumber(ctx, nil); err == nil {
			if id, err := db.CreateListenerSession(ctx, el.networkId, header.Number.Int64()); err == nil {
				sessionId = id
			}
		}
	}
	if sessionId == "" {
		evmLog.Warn("could not create listener session at startup, continuing without session tracking")
	}

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			evmLog.Info("shutting down block listener")
			if sessionId != "" {
				// ctx is already cancelled — use a fresh context for the shutdown write.
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := db.CloseListenerSession(shutdownCtx, sessionId, lastProcessedBlock); err != nil {
					evmLog.Error("failed to close listener session on shutdown", "sessionId", sessionId, "error", err)
				}
			}
			return
		case <-ticker.C:

			shouldRetry, nextProviderRetry = requireRetry(nextProviderRetry)
			if shouldRetry {
				recoverUnhealthyProviders(ctx, el.providerList)
			}

			client, provider, ok := getHealthyClient(el.providerList)
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

			safeBlock := new(big.Int).Sub(header.Number, big.NewInt(el.safeBlockBuffer))

			indexedAddresses, err := db.GetIndexedAddressToMonitor(ctx, el.networkId)
			if err != nil {
				evmLog.Error("failed to get indexed addresses", "error", err)
				continue
			}
			var addressesToMonitor []common.Address
			for _, idxAddress := range indexedAddresses {
				addressesToMonitor = append(addressesToMonitor, common.HexToAddress(idxAddress.WalletAddress))
			}

			if len(addressesToMonitor) == 0 {
				evmLog.Info("no indexed addresses to monitor, advancing block pointer", "safeBlock", safeBlock)
				if err := db.UpdateLastScannedBlock(ctx, el.networkId, safeBlock); err != nil {
					evmLog.Error("failed to advance block pointer", "error", err)
				}
				continue
			}

			lastBlock, err := db.GetLastScannedBlock(ctx, el.networkId)
			if err != nil {
				evmLog.Error("failed to get last scanned block", "error", err)
				continue
			}

			var from *big.Int
			if lastBlock.Sign() == 0 {
				from = safeBlock
			} else {
				from = new(big.Int).Add(lastBlock, big.NewInt(1))
			}

			if from.Cmp(safeBlock) > 0 {
				evmLog.Info("no new confirmed blocks", "lastBlock", lastBlock, "safeBlock", safeBlock)
				continue
			}

			// Cap to maxBlocksPerTick — prevents unbounded catchup on restart.
			cappedEnd := new(big.Int).Add(from, big.NewInt(el.maxBlocksPerTick-1))
			if cappedEnd.Cmp(safeBlock) < 0 {
				safeBlock = cappedEnd
			}

			evmLog.Info("processing block range", "from", from, "to", safeBlock)
			start := time.Now()

			for blockNum := new(big.Int).Set(from); blockNum.Cmp(safeBlock) <= 0; blockNum.Add(blockNum, big.NewInt(1)) {
				block, err := client.BlockByNumber(ctx, new(big.Int).Set(blockNum))
				if err != nil {
					evmLog.Error("failed to fetch block", "block", blockNum, "provider", provider.URL(), "error", err)
					handleProviderFailure(provider, err)
					break
				}
				printTxns(block.Number(), signer, block.Transactions(), addressesToMonitor)
				if el.usdcListen {
					checkUSDCTransferWithLogs(ctx, client, block, addressesToMonitor)
				}
				lastBlock = new(big.Int).Set(blockNum)
			}

			if err := db.UpdateLastScannedBlock(ctx, el.networkId, lastBlock); err != nil {
				evmLog.Error("failed to persist last scanned block", "lastBlock", lastBlock, "error", err)
			} else {
				lastProcessedBlock = lastBlock
			}
			evmLog.Info("finished processing blocks", "lastBlock", lastBlock, "duration", time.Since(start).Round(time.Millisecond))

		}
	}
}

func printTxns(blockNum *big.Int, signer types.Signer, txns types.Transactions, addresses []common.Address) {
	for _, tx := range txns {
		for _, addr := range addresses {
			if tx.To() != nil && *tx.To() == addr {
				evmLog.Info("incoming txn",
					"block", blockNum,
					"tx", tx.Hash().Hex(),
					"to", tx.To().Hex(),
					"value", tx.Value().String(),
				)
			}
		}

		sender, err := types.Sender(signer, tx)
		if err != nil {
			evmLog.Error("failed to recover sender", "block", blockNum, "tx", tx.Hash().Hex(), "error", err)
			continue
		}
		for _, addr := range addresses {
			if sender == addr {
				logOutgoingTxn(blockNum, sender, tx)
			}
		}
	}
}

func logOutgoingTxn(blockNum *big.Int, sender common.Address, tx *types.Transaction) {
	base := []any{
		"block", blockNum,
		"tx", tx.Hash().Hex(),
		"from", sender.Hex(),
		"value", tx.Value().String(),
	}

	switch {
	case tx.To() == nil: // contract deployment
		evmLog.Info("outgoing txn: contract deployment", base...)
	case len(tx.Data()) == 0: // Simple ETH Transfer
		evmLog.Info("outgoing txn: ETH transfer",
			append(base, "to", tx.To().Hex())...)
	case tx.Value().Sign() > 0: // Contract call with ETH payable function state change
		evmLog.Info("outgoing txn: contract call with ETH",
			append(base, "to", tx.To().Hex(), "selector", fmt.Sprintf("0x%x", tx.Data()[:4]))...)
	default: // Contract call with no ETH but gas fee to pay, state change only
		evmLog.Info("outgoing txn: contract call",
			append(base, "to", tx.To().Hex(), "selector", fmt.Sprintf("0x%x", tx.Data()[:4]))...)
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
