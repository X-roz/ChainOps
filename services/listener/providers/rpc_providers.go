package providers

import (
	"fmt"

	"github.com/ethereum/go-ethereum/ethclient"
)

type ProviderStatus string

const (
	Unhealthy ProviderStatus = "unhealthy"
	Healthy   ProviderStatus = "healthy"
)

type RPCProvider struct {
	Url          string
	Client       *ethclient.Client
	FailureCount int
	Status       ProviderStatus
}

func Connect(urls *[]string) ([]RPCProvider, error) {

	var ProviderList []RPCProvider

	for _, url := range *urls {
		provider := RPCProvider{
			Url:          url,
			FailureCount: 0,
			Status:       Healthy,
		}

		client, err := ethclient.Dial(provider.Url)
		if err != nil {
			fmt.Printf("Error connecting to provider: %s, Error: %v\n", provider.Url, err)
			return nil, err
		}
		provider.Client = client
		provider.Status = Healthy
		fmt.Printf("Successfully connected to provider: %s\n", provider.Url)

		ProviderList = append(ProviderList, provider)
	}

	fmt.Printf("Total providers connected: %d\n", len(ProviderList))
	return ProviderList, nil
}
