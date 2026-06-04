-- Tracks each listener run as a session; used to detect gaps and drive backfill
CREATE TABLE network_listener_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    network_id UUID NOT NULL REFERENCES networks(id),
    from_block BIGINT NOT NULL,
    to_block BIGINT,
    status VARCHAR(10) NOT NULL DEFAULT 'ACTIVE',
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,
    CONSTRAINT chk_session_status CHECK (status IN ('ACTIVE', 'CLOSED'))
);