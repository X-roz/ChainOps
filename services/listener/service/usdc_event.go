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

var usdcAddress = common.HexToAddress("0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238")

var transferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

var usdcAsset = &schema.Asset{
	AssetType:       "ERC20",
	Symbol:          "USDC",
	ContractAddress: usdcAddress.Hex(),
}

func collectUSDCEvents(ctx context.Context, client *ethclient.Client, block *types.Block, addresses []common.Address) []schema.ActivityEvent {
	if len(addresses) == 0 {
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

	var events []schema.ActivityEvent
	for _, vlog := range logs {
		events = append(events, collectUSDCLogEvents(vlog, addresses)...)
	}
	return events
}

func collectUSDCLogEvents(vlog types.Log, addresses []common.Address) []schema.ActivityEvent {
	from := common.BytesToAddress(vlog.Topics[1].Bytes())
	to := common.BytesToAddress(vlog.Topics[2].Bytes())
	amount := new(big.Int).SetBytes(vlog.Data).String()
	zero := common.Address{}

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
			}
		}
		if event != nil {
			events = append(events, *event)
		}
	}
	return events
}
