CREATE TABLE wallet_activities (
    -- Unique activity identifier
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- References the monitored wallet
    indexed_wallet_id UUID NOT NULL,
    -- Blockchain transaction hash
    tx_hash VARCHAR(100) NOT NULL,
    -- Network block/slot number where activity occurred
    block_number BIGINT NOT NULL,
    -- Block timestamp from chain
    block_timestamp TIMESTAMP NOT NULL,
    -- Activity category
    -- Examples:
    -- NATIVE_TRANSFER
    -- TOKEN_TRANSFER
    -- CONTRACT_INTERACTION
    -- CONTRACT_DEPLOYMENT
    event_type VARCHAR(50) NOT NULL,
    -- Wallet perspective
    -- Examples:
    -- INCOMING
    -- OUTGOING
    -- MINT
    -- BURN
    activity_type VARCHAR(50) NOT NULL,
    -- Sender address
    from_address VARCHAR(100),
    -- Receiver address
    to_address VARCHAR(100),
    -- Asset quantity
    amount NUMERIC(38, 18),
    -- Asset metadata
    asset_type VARCHAR(50),
    asset_symbol VARCHAR(50),
    asset_contract_address VARCHAR(100),
    -- Total fee paid by transaction sender
    fee_paid NUMERIC(38, 18),
    -- ETH, MATIC, SOL, etc.
    fee_asset VARCHAR(20),
    -- Event-specific information
    -- Examples:
    -- {
    --   "status": 1,
    --   "contract_address": "0x..."
    -- }
    --
    -- {
    --   "selector": "0xa9059cbb",
    --   "status": 1
    -- }
    --
    -- {
    --   "gas_used": 21000,
    --   "effective_gas_price": "20000000000"
    -- }
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_wallet_activities_wallet FOREIGN KEY (indexed_wallet_id) REFERENCES indexed_wallets(id) ON DELETE CASCADE
);