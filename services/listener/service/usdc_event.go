package service

import (
	"context"
	"listener/providers"
	"log/slog"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var usdcAddress = common.HexToAddress("0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238")

func USDCEventListener(ctx context.Context, subscriberList []*providers.EVMProvider) {
	transferTopic := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

	usdcQuery := ethereum.FilterQuery{
		Addresses: []common.Address{usdcAddress},
		Topics:    [][]common.Hash{{transferTopic}},
	}

	logs := make(chan types.Log)

	sub, err := subscriberList[0].Client().SubscribeFilterLogs(ctx, usdcQuery, logs)
	if err != nil {
		slog.Error("failed to subscribe to USDC transfer events", "error", err)
		return
	}
	slog.Info("subscribed to USDC transfer events")

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutting down USDC event listener")
			return
		case err := <-sub.Err():
			slog.Error("USDC subscription error", "error", err)
			handleProviderFailure(subscriberList[0], err)
			return
		case vlog := <-logs:
			slog.Info("USDC transfer event",
				"block", vlog.BlockNumber,
				"txHash", vlog.TxHash.String(),
				"from", common.BytesToAddress(vlog.Topics[1].Bytes()),
				"to", common.BytesToAddress(vlog.Topics[2].Bytes()),
				"value", new(big.Int).SetBytes(vlog.Data).String(),
			)
		}
	}
}
