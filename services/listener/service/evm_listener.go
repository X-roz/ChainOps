package service

import (
	"context"
	"errors"
	"fmt"
	"listener/db"
	_ "listener/logger"
	"listener/providers"
	"listener/schema"
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

var ethAsset = &schema.Asset{}

type EvmListener struct {
	providerList     []*providers.EVMProvider
	nativeAsset      string
	safeBlockBuffer  int64
	maxBlocksPerTick int64
	usdcListen       bool
	networkId        string
}

func NewEvmListener(providerList []*providers.EVMProvider, safeBlockBuffer int64, maxBlocksPerTick int64, usdcListen bool, networkId string, nativeAsset string) *EvmListener {
	ethAsset.AssetType = "NATIVE"
	ethAsset.Symbol = nativeAsset
	return &EvmListener{
		providerList:     providerList,
		safeBlockBuffer:  safeBlockBuffer,
		maxBlocksPerTick: maxBlocksPerTick,
		usdcListen:       usdcListen,
		networkId:        networkId,
		nativeAsset:      nativeAsset,
	}
}

func (el *EvmListener) Run(ctx context.Context) {

	signer := types.LatestSignerForChainID(el.providerList[0].ChainID())
	nextProviderRetry := time.Now().Add(5 * time.Minute)
	var shouldRetry bool
	var lastBlock *big.Int // last successfully processed block — used for session close
	var fromBlock *big.Int // start of next tick's range — advances after every tick

	var sessionId string
	if client, _, ok := getHealthyClient(el.providerList); ok {
		if header, err := client.HeaderByNumber(ctx, nil); err == nil {
			fromBlock = new(big.Int).Sub(header.Number, big.NewInt(el.safeBlockBuffer))
			if id, err := db.CreateListenerSession(ctx, el.networkId, fromBlock.Int64()); err == nil {
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
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := db.CloseListenerSession(shutdownCtx, sessionId, lastBlock); err != nil {
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
				lastBlock = new(big.Int).Set(safeBlock)
				fromBlock = new(big.Int).Add(safeBlock, big.NewInt(1))
				if err := db.UpdateLastScannedBlock(ctx, el.networkId, safeBlock); err != nil {
					evmLog.Error("failed to advance block pointer", "error", err)
				}
				continue
			}

			// fromBlock is set at startup; fall back to current safeBlock if session creation failed.
			from := fromBlock
			if from == nil {
				from = new(big.Int).Set(safeBlock)
			}

			if from.Cmp(safeBlock) > 0 {
				evmLog.Info("no new confirmed blocks", "from", from, "safeBlock", safeBlock)
				continue
			}

			cappedEnd := new(big.Int).Add(from, big.NewInt(el.maxBlocksPerTick-1))
			if cappedEnd.Cmp(safeBlock) < 0 {
				safeBlock = cappedEnd
			}

			evmLog.Info("processing block range", "from", from, "to", safeBlock)
			start := time.Now()

			var totalEvents int
			for blockNum := new(big.Int).Set(from); blockNum.Cmp(safeBlock) <= 0; blockNum.Add(blockNum, big.NewInt(1)) {
				block, err := client.BlockByNumber(ctx, new(big.Int).Set(blockNum))
				if err != nil {
					evmLog.Error("failed to fetch block", "block", blockNum, "provider", provider.URL(), "error", err)
					handleProviderFailure(provider, err)
					break
				}

				msg := schema.BlockActivityMessage{
					NetworkID:      el.networkId,
					BlockNumber:    block.NumberU64(),
					BlockHash:      block.Hash().Hex(),
					BlockTimestamp: time.Unix(int64(block.Time()), 0),
				}
				msg.Events = append(msg.Events, collectTxnEvents(signer, client, block, addressesToMonitor)...)
				if el.usdcListen {
					msg.Events = append(msg.Events, collectUSDCEvents(ctx, client, block, addressesToMonitor)...)
				}
				if len(msg.Events) > 0 {
					evmLog.Info("block activity detected", "block", msg.BlockNumber, "events", len(msg.Events))
				}
				totalEvents += len(msg.Events)
				lastBlock = new(big.Int).Set(blockNum)
			}

			if lastBlock != nil {
				fromBlock = new(big.Int).Add(lastBlock, big.NewInt(1))
				if err := db.UpdateLastScannedBlock(ctx, el.networkId, lastBlock); err != nil {
					evmLog.Error("failed to persist last scanned block", "lastBlock", lastBlock, "error", err)
				}
			}
			evmLog.Info("finished processing blocks", "lastBlock", lastBlock, "events", totalEvents, "duration", time.Since(start).Round(time.Millisecond))

		}
	}
}

func collectTxnEvents(signer types.Signer, client *ethclient.Client, block *types.Block, addresses []common.Address) []schema.ActivityEvent {
	var events []schema.ActivityEvent

	for _, tx := range block.Transactions() {
		for _, addr := range addresses {
			// Incoming native transfer: gas details Optional
			if tx.To() != nil && *tx.To() == addr {
				events = append(events, schema.ActivityEvent{
					WalletAddress: addr.Hex(),
					TxHash:        tx.Hash().Hex(),
					EventType:     schema.EventTypeNativeTransfer,
					ActivityType:  schema.ActivityTypeIncoming,
					ToAddress:     tx.To().Hex(),
					Amount:        tx.Value().String(),
					Asset:         ethAsset,
					GasDetails:    nil,
				})
			}
		}

		sender, err := types.Sender(signer, tx)
		if err != nil {
			evmLog.Error("failed to recover sender", "block", block.Number(), "tx", tx.Hash().Hex(), "error", err)
			continue
		}

		// Outgoing transactions (native transfer, contract interaction, or deployment): gas details required
		for _, addr := range addresses {
			if sender != addr {
				continue
			}
			gasDetails := &schema.GasDetails{}

			transactionReceipt := getReceiptTransaction(context.Background(), client, tx.Hash())
			if transactionReceipt != nil {
				gasDetails = getGasDetails(transactionReceipt)
			}

			base := schema.ActivityEvent{
				WalletAddress: addr.Hex(),
				TxHash:        tx.Hash().Hex(),
				ActivityType:  schema.ActivityTypeOutgoing,
				FromAddress:   sender.Hex(),
				Amount:        tx.Value().String(),
				Asset:         ethAsset,
				GasDetails:    gasDetails,
			}
			switch {
			case tx.To() == nil:
				base.EventType = schema.EventTypeContractDeployment
				if transactionReceipt != nil {
					base.Metadata = map[string]any{
						"status":           transactionReceipt.Status,
						"contract_address": transactionReceipt.ContractAddress.Hex(),
					}
				}
			case len(tx.Data()) == 0:
				base.EventType = schema.EventTypeNativeTransfer
				base.ToAddress = tx.To().Hex()
			default:
				base.EventType = schema.EventTypeContractInteraction
				base.ToAddress = tx.To().Hex()
				if transactionReceipt != nil {
					base.Metadata = map[string]any{
						"selector": fmt.Sprintf("0x%x", tx.Data()[:4]),
						"status":   transactionReceipt.Status,
					}
				} else {
					base.Metadata = map[string]any{"selector": fmt.Sprintf("0x%x", tx.Data()[:4])}
				}
			}
			events = append(events, base)
		}
	}
	return events
}

func getGasDetails(receipt *types.Receipt) *schema.GasDetails {
	return &schema.GasDetails{
		FeePaid:           new(big.Int).Mul(receipt.EffectiveGasPrice, new(big.Int).SetUint64(receipt.GasUsed)).String(),
		FeeAsset:          ethAsset.Symbol,
		GasUsed:           receipt.GasUsed,
		EffectiveGasPrice: receipt.EffectiveGasPrice.String(),
	}
}

func getReceiptTransaction(ctx context.Context, client *ethclient.Client, txHash common.Hash) *types.Receipt {
	receipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		evmLog.Error("failed to fetch transaction receipt", "txHash", txHash.Hex(), "error", err)
		return nil
	}
	return receipt
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
