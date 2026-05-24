package main

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {

	fmt.Println("Starting Ethereum client listener...")

	// Connect to the Ethereum client
	client, err := ethclient.Dial("https://0xrpc.io/sep")
	if err != nil {
		fmt.Println("Error connecting to Ethereum client:", err)
		return
	}

	networkId, err := client.NetworkID(context.Background())
	if err != nil {
		fmt.Println("Error in fetching network Id :", err)
		return
	}

	fmt.Println("Connected to Ethereum client with network ID:", networkId)

	blockNumber, err := client.BlockNumber(context.Background())
	if err != nil {
		fmt.Println("Error in fetching block number :", err)
		return
	}
	fmt.Println("Current block number:", blockNumber)

	bn := big.NewInt(int64(blockNumber))

	block, err := client.BlockByNumber(context.Background(), bn)
	if err != nil {
		fmt.Println("Error in fetching block :", err)
		return
	}
	fmt.Println("No. of Transactions:", len(block.Transactions()))

	for _, tx := range block.Transactions() {

		sender, err := types.Sender(types.LatestSignerForChainID(networkId), tx)
		if err != nil {
			fmt.Println("Error in fetching sender :", err)
			return
		}

		to := "contract creation"
		if tx.To() != nil {
			to = tx.To().Hex()
		}
		fmt.Printf("Transaction Hash: %s, From: %s, To: %s, Value: %s\n", tx.Hash().Hex(), sender.Hex(), to, tx.Value().String())
	}

	client.Close()
}
