package service

import (
	"context"
	"listener/providers"
	"log/slog"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var usdcLog = slog.With("listener", "[usdc_event]")

var usdcAddress = common.HexToAddress("0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238")

var transferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

func USDCEventListener(ctx context.Context, subscriberList []*providers.EVMProvider, safeBlockBuffer int64) {
	// lastBlock survives reconnects within the process.
	// Cross-restart persistence requires a DB.
	var lastBlock uint64

	for {
		provider, ok := getHealthySubscriber(subscriberList)
		if !ok {
			usdcLog.Error("no healthy subscriber providers available")
			select {
			case <-ctx.Done():
				usdcLog.Info("shutting down USDC event listener")
				return
			case <-time.After(10 * time.Second):
				continue
			}
		}

		if lastBlock > 0 {
			caught, err := catchUpUSDCLogs(ctx, provider.Client(), lastBlock+1, safeBlockBuffer)
			if err != nil {
				usdcLog.Error("USDC catch-up failed", "fromBlock", lastBlock+1, "error", err)
				provider.RecordFailure()
			} else {
				lastBlock = caught
			}
		}

		lastBlock = liveSubscribeUSDC(ctx, provider, lastBlock)

		select {
		case <-ctx.Done():
			usdcLog.Info("shutting down USDC event listener")
			return
		case <-time.After(5 * time.Second):
			usdcLog.Info("retrying USDC subscription", "fromBlock", lastBlock+1)
		}
	}
}

func getHealthySubscriber(subscriberList []*providers.EVMProvider) (*providers.EVMProvider, bool) {
	for _, p := range subscriberList {
		if p.IsHealthy() {
			return p, true
		}
	}
	return nil, false
}

// catchUpUSDCLogs fetches missed events between fromBlock and head-safeBlockBuffer
// using a one-shot HTTP FilterLogs call, returning the latest block processed.
func catchUpUSDCLogs(ctx context.Context, client *ethclient.Client, fromBlock uint64, safeBlockBuffer int64) (uint64, error) {
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return fromBlock - 1, err
	}

	headBlock := header.Number.Uint64()
	if uint64(safeBlockBuffer) >= headBlock {
		return fromBlock - 1, nil
	}
	toBlock := headBlock - uint64(safeBlockBuffer)

	if fromBlock > toBlock {
		return fromBlock - 1, nil
	}

	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(fromBlock),
		ToBlock:   new(big.Int).SetUint64(toBlock),
		Addresses: []common.Address{usdcAddress},
		Topics:    [][]common.Hash{{transferTopic}},
	}

	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		return fromBlock - 1, err
	}

	usdcLog.Info("USDC catch-up complete", "fromBlock", fromBlock, "toBlock", toBlock, "events", len(logs))
	for _, vlog := range logs {
		processUSDCLog(vlog)
	}
	return toBlock, nil
}

func liveSubscribeUSDC(ctx context.Context, provider *providers.EVMProvider, lastBlock uint64) uint64 {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{usdcAddress},
		Topics:    [][]common.Hash{{transferTopic}},
	}

	logs := make(chan types.Log)

	sub, err := provider.Client().SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		usdcLog.Error("failed to subscribe to USDC transfer events", "provider", provider.URL(), "error", err)
		provider.RecordFailure()
		return lastBlock
	}
	usdcLog.Info("subscribed to USDC transfer events", "provider", provider.URL())

	for {
		select {
		case <-ctx.Done():
			return lastBlock
		case err := <-sub.Err():
			usdcLog.Error("USDC subscription dropped", "provider", provider.URL(), "error", err)
			handleProviderFailure(provider, err)
			return lastBlock
		case vlog := <-logs:
			processUSDCLog(vlog)
			if !vlog.Removed {
				lastBlock = vlog.BlockNumber
			}
		}
	}
}

func processUSDCLog(vlog types.Log) {
	if vlog.Removed {
		usdcLog.Warn("USDC transfer reorged out",
			"block", vlog.BlockNumber,
			"txHash", vlog.TxHash.String(),
		)
		return
	}

	from := common.BytesToAddress(vlog.Topics[1].Bytes())
	to := common.BytesToAddress(vlog.Topics[2].Bytes())
	value := new(big.Int).SetBytes(vlog.Data).String()
	zero := common.Address{}

	switch {
	case to == addressToMonitor && from == zero:
		usdcLog.Info("USDC mint",
			"block", vlog.BlockNumber,
			"txHash", vlog.TxHash.String(),
			"to", to,
			"value", value,
		)
	case to == addressToMonitor:
		usdcLog.Info("USDC incoming transfer",
			"block", vlog.BlockNumber,
			"txHash", vlog.TxHash.String(),
			"from", from,
			"to", to,
			"value", value,
		)
	case from == addressToMonitor && to == zero:
		usdcLog.Info("USDC burn",
			"block", vlog.BlockNumber,
			"txHash", vlog.TxHash.String(),
			"from", from,
			"value", value,
		)
	case from == addressToMonitor:
		usdcLog.Info("USDC outgoing transfer",
			"block", vlog.BlockNumber,
			"txHash", vlog.TxHash.String(),
			"from", from,
			"to", to,
			"value", value,
		)
	}
}
