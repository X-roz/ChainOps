package schema

import "time"

type EventType string

const (
	EventTypeNativeTransfer EventType = "NATIVE_TRANSFER"

	EventTypeTokenTransfer EventType = "TOKEN_TRANSFER"

	EventTypeContractInteraction EventType = "CONTRACT_INTERACTION"

	EventTypeContractDeployment EventType = "CONTRACT_DEPLOYMENT"

	EventTypeNFTTransfer EventType = "NFT_TRANSFER"

	EventTypeDeFiSwap EventType = "DEFI_SWAP"
)

type ActivityType string

const (
	ActivityTypeIncoming ActivityType = "INCOMING"

	ActivityTypeOutgoing ActivityType = "OUTGOING"

	ActivityTypeMint ActivityType = "MINT"

	ActivityTypeBurn ActivityType = "BURN"
)

type Asset struct {
	AssetType       string `json:"type"`
	Symbol          string `json:"symbol"`
	ContractAddress string `json:"contract_address,omitempty"`
}

type ActivityEvent struct {
	WalletAddress string         `json:"wallet_address"`
	TxHash        string         `json:"tx_hash"`
	EventType     EventType      `json:"event_type"`
	ActivityType  ActivityType   `json:"activity_type"`
	FromAddress   string         `json:"from_address"`
	ToAddress     string         `json:"to_address"`
	Amount        string         `json:"amount"`
	Asset         *Asset         `json:"asset,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type BlockActivityMessage struct {
	NetworkID      string          `json:"network_id"`
	BlockNumber    uint64          `json:"block_number"`
	BlockHash      string          `json:"block_hash"`
	BlockTimestamp time.Time       `json:"block_timestamp"`
	Events         []ActivityEvent `json:"events"`
}
