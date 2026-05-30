-- Executed on 2026-05-30
CREATE TABLE networks (
    id UUID PRIMARY KEY,
    network_key VARCHAR(50) NOT NULL UNIQUE,
    display_name VARCHAR(100) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
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
CREATE TABLE indexed_wallets (
    wallet_address VARCHAR(100) NOT NULL,
    network_id UUID NOT NULL REFERENCES networks(id),
    active_subscriber_count INTEGER NOT NULL DEFAULT 0,
    last_scanned_block BIGINT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (wallet_address, network_id)
);
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