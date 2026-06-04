package service

import (
	"context"
	"log/slog"
	"math/big"

	"listener/schema"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var usdcLog = slog.With("listener", "[usdc_event]")

var transferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

var (
	usdcAddress common.Address
	usdcAsset   *schema.Asset
)

// InitTokenContracts populates token contract addresses from config once at startup.
// Keyed by symbol (e.g. "USDC") so individual processors can look up their address
// without parsing the full map on every tick.
func InitTokenContracts(contracts map[string]string) {
	addr, ok := contracts["USDC"]
	if !ok {
		usdcLog.Warn("USDC not found in known-token-contracts; USDC event collection will be skipped")
		return
	}
	usdcAddress = common.HexToAddress(addr)
	usdcAsset = &schema.Asset{
		AssetType:       "ERC20",
		Symbol:          "USDC",
		ContractAddress: usdcAddress.Hex(),
	}
}

func collectUSDCEvents(ctx context.Context, client *ethclient.Client, block *types.Block, addresses []common.Address) []schema.ActivityEvent {
	if usdcAsset == nil || len(addresses) == 0 {
		return nil
	}

	logs, err := client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: block.Number(),
		ToBlock:   block.Number(),
		Addresses: []common.Address{usdcAddress},
		Topics:    [][]common.Hash{{transferTopic}},
	})
	if err != nil {
		usdcLog.Error("failed to filter USDC logs", "block", block.Number(), "error", err)
		return nil
	}

	// Build a set of monitored addresses for O(1) lookup.
	addrSet := make(map[common.Address]struct{}, len(addresses))
	for _, a := range addresses {
		addrSet[a] = struct{}{}
	}

	// Pre-fetch receipts only for transactions where a monitored wallet is the
	// sender. Receipts carry gas details that are attached to outgoing events.
	receipts := make(map[common.Hash]*types.Receipt)

	for _, vlog := range logs {
		if _, alreadyFetched := receipts[vlog.TxHash]; alreadyFetched {
			continue
		}
		from := common.BytesToAddress(vlog.Topics[1].Bytes())
		if _, isMonitored := addrSet[from]; !isMonitored {
			continue
		}
		receipt, err := client.TransactionReceipt(ctx, vlog.TxHash)
		if err != nil {
			usdcLog.Warn("failed to fetch receipt for outgoing tx", "tx", vlog.TxHash.Hex(), "error", err)
			receipts[vlog.TxHash] = nil // mark attempted so we don't retry
			continue
		}
		receipts[vlog.TxHash] = receipt
	}

	var events []schema.ActivityEvent
	for _, vlog := range logs {
		events = append(events, collectUSDCLogEvents(vlog, addresses, receipts[vlog.TxHash])...)
	}
	return events
}

func collectUSDCLogEvents(vlog types.Log, addresses []common.Address, receipt *types.Receipt) []schema.ActivityEvent {
	from := common.BytesToAddress(vlog.Topics[1].Bytes())
	to := common.BytesToAddress(vlog.Topics[2].Bytes())
	amount := new(big.Int).SetBytes(vlog.Data).String()
	zero := common.Address{}

	// log_index and tx_index allow consumers to reconstruct the exact position of
	// this event within the block without re-fetching the receipt.
	metadata := map[string]any{
		"log_index": vlog.Index,
		"tx_index":  vlog.TxIndex,
	}

	var gasDetails *schema.GasDetails
	if receipt != nil {
		gasDetails = buildGasDetails(receipt)
	}

	var events []schema.ActivityEvent
	for _, addr := range addresses {
		var event *schema.ActivityEvent
		switch {
		case to == addr && from == zero:
			event = &schema.ActivityEvent{
				WalletAddress: addr.Hex(),
				TxHash:        vlog.TxHash.Hex(),
				EventType:     schema.EventTypeTokenTransfer,
				ActivityType:  schema.ActivityTypeMint,
				FromAddress:   from.Hex(),
				ToAddress:     to.Hex(),
				Amount:        amount,
				Asset:         usdcAsset,
				Metadata:      metadata,
			}
		case to == addr:
			event = &schema.ActivityEvent{
				WalletAddress: addr.Hex(),
				TxHash:        vlog.TxHash.Hex(),
				EventType:     schema.EventTypeTokenTransfer,
				ActivityType:  schema.ActivityTypeIncoming,
				FromAddress:   from.Hex(),
				ToAddress:     to.Hex(),
				Amount:        amount,
				Asset:         usdcAsset,
				Metadata:      metadata,
			}
		case from == addr && to == zero:
			event = &schema.ActivityEvent{
				WalletAddress: addr.Hex(),
				TxHash:        vlog.TxHash.Hex(),
				EventType:     schema.EventTypeTokenTransfer,
				ActivityType:  schema.ActivityTypeBurn,
				FromAddress:   from.Hex(),
				ToAddress:     to.Hex(),
				Amount:        amount,
				Asset:         usdcAsset,
				GasDetails:    gasDetails,
				Metadata:      metadata,
			}
		case from == addr:
			event = &schema.ActivityEvent{
				WalletAddress: addr.Hex(),
				TxHash:        vlog.TxHash.Hex(),
				EventType:     schema.EventTypeTokenTransfer,
				ActivityType:  schema.ActivityTypeOutgoing,
				FromAddress:   from.Hex(),
				ToAddress:     to.Hex(),
				Amount:        amount,
				Asset:         usdcAsset,
				GasDetails:    gasDetails,
				Metadata:      metadata,
			}
		}
		if event != nil {
			events = append(events, *event)
		}
	}
	return events
}

// buildGasDetails derives gas cost fields from a transaction receipt.
// fee_paid is denominated in the network's native currency (wei for EVM chains).
func buildGasDetails(receipt *types.Receipt) *schema.GasDetails {
	effectiveGasPrice := receipt.EffectiveGasPrice
	if effectiveGasPrice == nil {
		effectiveGasPrice = new(big.Int)
	}
	feePaid := new(big.Int).Mul(effectiveGasPrice, new(big.Int).SetUint64(receipt.GasUsed))
	return &schema.GasDetails{
		FeePaid:           feePaid.String(),
		FeeAsset:          "ETH",
		GasUsed:           receipt.GasUsed,
		EffectiveGasPrice: effectiveGasPrice.String(),
	}
}
