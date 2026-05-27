package providers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"

	"github.com/ethereum/go-ethereum/ethclient"
)

type EVMProvider struct {
	url          string
	client       *ethclient.Client
	failureCount int
	chainID      *big.Int
	status       ProviderStatus
}

func (e *EVMProvider) IsHealthy() bool {
	return e.status != Unhealthy
}

func (e *EVMProvider) URL() string {
	return e.url
}

// RecordFailure increments the failure count and marks the provider unhealthy at threshold 3.
func (e *EVMProvider) RecordFailure() {
	e.failureCount++
	if e.failureCount >= 3 {
		slog.Warn("provider marked unhealthy", "url", e.url, "failureCount", e.failureCount)
		e.status = Unhealthy
	}
}

func (e *EVMProvider) Recover(ctx context.Context) {
	_, err := e.client.BlockNumber(ctx)
	if err != nil {
		slog.Warn("provider still unhealthy", "url", e.url, "error", err)
		return
	}
	e.failureCount = 0
	e.status = Healthy
	slog.Info("provider recovered", "url", e.url)
}

func (e *EVMProvider) Client() *ethclient.Client {
	return e.client
}

func (e *EVMProvider) ChainID() *big.Int {
	return e.chainID
}

// ConnectEVM dials each URL, fetches its chain ID, validates all providers are on
// the same network, and returns a ready-to-use provider list.
func ConnectEVM(ctx context.Context, urls []string) ([]*EVMProvider, error) {
	if len(urls) == 0 {
		return nil, errors.New("no RPC URLs provided")
	}

	var providerList []*EVMProvider
	var expectedChainID *big.Int

	for _, url := range urls {
		client, err := ethclient.DialContext(ctx, url)
		if err != nil {
			slog.Error("failed to connect to provider", "url", url, "error", err)
			return nil, err
		}

		chainID, err := client.ChainID(ctx)
		if err != nil {
			slog.Error("failed to fetch chain ID", "url", url, "error", err)
			return nil, err
		}

		if expectedChainID == nil {
			expectedChainID = chainID
		} else if chainID.Cmp(expectedChainID) != 0 {
			return nil, fmt.Errorf("chain ID mismatch: provider %s returned %s, expected %s", url, chainID, expectedChainID)
		}

		providerList = append(providerList, &EVMProvider{
			url:     url,
			client:  client,
			status:  Healthy,
			chainID: chainID,
		})
		slog.Info("connected to provider", "url", url, "chainID", chainID)
	}

	slog.Info("all providers connected", "count", len(providerList), "chainID", expectedChainID)
	return providerList, nil
}
