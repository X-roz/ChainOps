package providers

import "context"

type ProviderStatus string

const (
	Healthy   ProviderStatus = "healthy"
	Unhealthy ProviderStatus = "unhealthy"
)

// Provider is the shared contract across chain types.
// Concrete types (EVMProvider, SolanaProvider) add chain-specific methods on top.
type Provider interface {
	IsHealthy() bool
	RecordFailure()
	Recover(ctx context.Context)
	URL() string
}
