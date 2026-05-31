-- Drop the old indexed_wallets table before recreating with the updated schema
drop table indexed_wallets;

-- Tracks each monitoring session for an indexed wallet, recording the block range
-- over which the wallet was actively listened to. A wallet can have multiple
-- sequential sessions (session_number is monotonically increasing per wallet).
CREATE TABLE wallet_monitoring_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    indexed_wallet_id UUID NOT NULL,
    session_number INTEGER NOT NULL,
    started_block BIGINT NOT NULL,
    ended_block BIGINT,                         -- NULL while the session is still active
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMP,                         -- NULL while the session is still active
    CONSTRAINT fk_monitoring_session_wallet FOREIGN KEY (indexed_wallet_id) REFERENCES indexed_wallets(id) ON DELETE CASCADE,
    CONSTRAINT uq_wallet_session UNIQUE (indexed_wallet_id, session_number)
);

-- Represents a blockchain wallet address that ChainOps is monitoring on a specific
-- network. The unique constraint on (wallet_address, network_id) prevents the same
-- wallet from being indexed twice on the same network.
CREATE TABLE indexed_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_address VARCHAR(100) NOT NULL,
    network_id UUID NOT NULL,
    active_subscriber_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_wallet_network UNIQUE (wallet_address, network_id),
    CONSTRAINT fk_indexed_wallet_network FOREIGN KEY (network_id) REFERENCES networks(id) ON DELETE CASCADE
);

-- Lifecycle states for a monitoring session:
--   OPEN      - session created, not yet receiving events
--   LISTENING - actively consuming blockchain events
--   CLOSED    - session ended (block range is fully recorded)
CREATE TYPE monitoring_session_status AS ENUM ('OPEN', 'LISTENING', 'CLOSED');

-- Add the status column to existing sessions; defaults to OPEN for any
-- sessions created before this column existed
ALTER TABLE wallet_monitoring_sessions
ADD COLUMN status monitoring_session_status NOT NULL DEFAULT 'OPEN';

-- Seed: register a known wallet on the Sepolia test network with one active subscriber
INSERT INTO indexed_wallets (
        wallet_address,
        network_id,
        active_subscriber_count
    )
VALUES (
        '0x32056651573c19C329c9619DAF25A72e0D8a48dC',
        'a1b59dde-2714-4fa8-b2a8-92ab6bb51590',
        1
    )
RETURNING id;

-- Seed: open the first monitoring session for the wallet above, starting from block 0
INSERT INTO wallet_monitoring_sessions (
        indexed_wallet_id,
        session_number,
        started_block,
        ended_block,
        status
    )
SELECT id,
    1,
    0,
    NULL,   -- no end block yet; session is open
    'OPEN'
FROM indexed_wallets
WHERE wallet_address = '0x32056651573c19C329c9619DAF25A72e0D8a48dC'
    AND network_id = 'a1b59dde-2714-4fa8-b2a8-92ab6bb51590';
