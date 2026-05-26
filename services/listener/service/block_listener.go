package service

import (
	"context"
	"fmt"
	"listener/providers"
	"math/big"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

var addressToMonitor = "0x32056651573c19C329c9619DAF25A72e0D8a48dC"

const confirmations = int64(12)

func ListenToBlocks(ctx context.Context, providerList *[]providers.RPCProvider) {
	// nil means first run; using nil instead of 0 avoids ambiguity with genesis block
	var lastBlock *big.Int

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Shutting down block listener...")
			return
		case <-ticker.C:
			client, provider, ok := getHealthyClient(providerList)
			if !ok {
				fmt.Println("No healthy providers available, skipping tick")
				continue
			}

			header, err := client.HeaderByNumber(ctx, nil)
			if err != nil {
				fmt.Printf("Error fetching latest block from %s: %v\n", provider.Url, err)
				handleProviderFailure(provider, err)
				continue
			}

			safeBlock := new(big.Int).Sub(header.Number, big.NewInt(confirmations))

			var from *big.Int
			if lastBlock == nil {
				// First run: start at safeBlock (latest - 12)
				from = safeBlock
			} else {
				from = new(big.Int).Add(lastBlock, big.NewInt(1))
			}

			if from.Cmp(safeBlock) > 0 {
				fmt.Printf("No new confirmed blocks (lastBlock=%s, safeBlock=%s)\n",
					lastBlock.String(), safeBlock.String())
				continue
			}

			fmt.Printf("Processing blocks %s → %s\n", from.String(), safeBlock.String())

			for blockNum := new(big.Int).Set(from); blockNum.Cmp(safeBlock) <= 0; blockNum.Add(blockNum, big.NewInt(1)) {
				block, err := client.BlockByNumber(ctx, new(big.Int).Set(blockNum))
				if err != nil {
					fmt.Printf("Error fetching block %s from %s: %v\n", blockNum.String(), provider.Url, err)
					handleProviderFailure(provider, err)
					break
				}
				fmt.Printf("Block %s | txns: %d\n", blockNum.String(), len(block.Transactions()))
				lastBlock = new(big.Int).Set(blockNum)
			}
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
		fmt.Printf("Provider %s failed %d times, marking unhealthy\n", provider.Url, provider.FailureCount)
		provider.Status = providers.Unhealthy
	}
}
