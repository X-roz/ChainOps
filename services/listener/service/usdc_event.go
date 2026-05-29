package service

import (
	"context"
	"log/slog"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var usdcLog = slog.With("listener", "[usdc_event]")

var usdcAddress = common.HexToAddress("0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238")

var transferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

func checkUSDCTransferWithLogs(ctx context.Context, client *ethclient.Client, block *types.Block) {
	query := ethereum.FilterQuery{
		FromBlock: block.Number(),
		ToBlock:   block.Number(),
		Addresses: []common.Address{usdcAddress},
		Topics:    [][]common.Hash{{transferTopic}},
	}

	logs, err := client.FilterLogs(ctx, query)
	if err != nil {
		usdcLog.Error("failed to filter USDC logs", "block", block.Number(), "error", err)
		return
	}

	for _, vlog := range logs {
		processUSDCLog(vlog)
	}
}

func processUSDCLog(vlog types.Log) {
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
