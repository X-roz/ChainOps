-- Executed on 2026-05-30
-- Stores supported blockchain networks with their keys and display metadata
CREATE TABLE networks (
    id UUID PRIMARY KEY,
    network_key VARCHAR(50) NOT NULL UNIQUE,
    display_name VARCHAR(100) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
-- Seed: register Ethereum mainnet as the first supported network
INSERT INTO networks (
        id,
        network_key,
        display_name,
        is_active
    )
VALUES (
        '550e8400-e29b-41d4-a716-446655440000',
        'ethereum',
        'Ethereum',
        TRUE
    );
-- Tracks wallet addresses being monitored per network, with scan progress
CREATE TABLE indexed_wallets (
    wallet_address VARCHAR(100) NOT NULL,
    network_id UUID NOT NULL REFERENCES networks(id),
    active_subscriber_count INTEGER NOT NULL DEFAULT 0,
    last_scanned_block BIGINT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (wallet_address, network_id)
);
-- Seed: start monitoring a USDC-related wallet on Ethereum mainnet
INSERT INTO indexed_wallets (
        wallet_address,
        network_id,
        active_subscriber_count,
        last_scanned_block
    )
SELECT '0x32056651573c19C329c9619DAF25A72e0D8a48dC',
    n.id,
    1,
    NULL
FROM networks n
WHERE n.network_key = 'ethereum';
-- Seed: register Sepolia testnet as a supported network
INSERT INTO networks (
        id,
        network_key,
        display_name,
        is_active
    )
VALUES (
        'a1b59dde-2714-4fa8-b2a8-92ab6bb51590',
        'sepolia',
        'Sepolia',
        TRUE
    );
-- Seed: start monitoring the same wallet on Sepolia testnet
INSERT INTO indexed_wallets (
        wallet_address,
        network_id,
        active_subscriber_count,
        last_scanned_block
    )
SELECT '0x32056651573c19C329c9619DAF25A72e0D8a48dC',
    n.id,
    1,
    NULL
FROM networks n
WHERE n.network_key = 'sepolia';
-- Tracks the highest block number scanned on this network; used to resume
-- listening from where the previous session left off instead of re-scanning from block 0
ALTER TABLE networks
ADD COLUMN last_scanned_block BIGINT NOT NULL DEFAULT 0;