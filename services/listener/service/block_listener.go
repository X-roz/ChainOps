package service

import (
	"context"
	"fmt"
	"listener/providers"
	"math/big"
	"net"

	"github.com/ethereum/go-ethereum/ethclient"
)

var addressToMonitor = "0x32056651573c19C329c9619DAF25A72e0D8a48dC"

func ListenToBlocks(ctx context.Context, providerList *[]providers.RPCProvider) {

	// Timer to fetch blocks every 1 minute and need one variable to hold the last block scanned.
	var lastBlock *big.Int = big.NewInt(0)

	// unlimited loop with reasonable sleep to avoid overwhelming the node
	for {

		for _, provider := range *providerList {

			if provider.Status == providers.Unhealthy {
				fmt.Printf("Skipping unhealthy provider: %s\n", provider.Url)
				continue
			}

			client := provider.Client

			blockToProcess, err := handleBlockProcess(ctx, client, lastBlock)
			if err != nil {
				fmt.Printf("Error processing block with provider %s: %v\n", provider.Url, err)
				handleProviderFailure(&provider, err)
				continue
			}

			// Process the block (placeholder for actual processing logic)
			block, err := client.BlockByNumber(ctx, blockToProcess)
			if err != nil {
				fmt.Printf("Error fetching block %s with provider %s: %v\n", blockToProcess.String(), provider.Url, err)
				handleProviderFailure(&provider, err)
				continue
			}

			fmt.Printf("Successfully processed block %s with provider %s\n", blockToProcess.String(), provider.Url)
			fmt.Printf("no. of transactions in block %s: %d\n", blockToProcess.String(), len(block.Transactions()))
		}

	}
}

func handleBlockProcess(ctx context.Context, client *ethclient.Client, lastBlock *big.Int) (*big.Int, error) {
	var blockToProcess *big.Int
	if lastBlock.Cmp(big.NewInt(0)) == 0 {
		header, err := client.HeaderByNumber(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest block header: %w", err)
		}
		blockToProcess = header.Number
		fmt.Printf("Starting from latest block: %s\n", blockToProcess.String())
		return blockToProcess, nil
	}
	blockToProcess = new(big.Int).Add(lastBlock, big.NewInt(1))
	return blockToProcess, nil
}

func handleProviderFailure(provider *providers.RPCProvider, err error) {
	if _, ok := err.(net.Error); !ok {
		return
	}
	fmt.Printf("Error with provider %s: %v\n", provider.Url, err)
	provider.FailureCount++
	if provider.FailureCount >= 3 {
		fmt.Printf("Provider %s has failed %d times. Consider removing it from the list.\n", provider.Url, provider.FailureCount)
		provider.Status = providers.Unhealthy
	}
}
