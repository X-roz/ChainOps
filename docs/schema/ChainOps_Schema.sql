-- =====================================================
-- USERS
-- =====================================================
CREATE TABLE users (
    id UUID PRIMARY KEY,
    primary_wallet_address VARCHAR(100) NOT NULL,
    current_plan VARCHAR(50) NOT NULL DEFAULT 'FREE_TRIAL',
    account_status VARCHAR(30) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMP
);
-- =====================================================
-- AUTH WALLETS
-- Wallets allowed to login via SIWE
-- =====================================================
CREATE TABLE auth_wallets (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    wallet_address VARCHAR(100) NOT NULL UNIQUE,
    role VARCHAR(20) NOT NULL DEFAULT 'OWNER',
    verified_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
-- =====================================================
-- NETWORKS
-- Ethereum, Base, Arbitrum, Optimism, etc.
-- =====================================================
CREATE TABLE networks (
    id UUID PRIMARY KEY,
    network_key VARCHAR(50) NOT NULL UNIQUE,
    display_name VARCHAR(100) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
-- =====================================================
-- ASSETS
-- ETH, USDC, USDT, etc.
-- =====================================================
CREATE TABLE assets (
    id UUID PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
-- =====================================================
-- ASSET DEPLOYMENTS
-- USDC/Base, USDC/Ethereum, ETH/Arbitrum, etc.
-- =====================================================
CREATE TABLE asset_deployments (
    id UUID PRIMARY KEY,
    asset_id UUID NOT NULL REFERENCES assets(id),
    network_id UUID NOT NULL REFERENCES networks(id),
    contract_address VARCHAR(100),
    decimals INTEGER NOT NULL,
    is_native BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(asset_id, network_id)
);
-- =====================================================
-- USER TRACKED WALLETS
-- User-level wallet subscriptions
-- =====================================================
CREATE TABLE user_tracked_wallets (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    wallet_address VARCHAR(100) NOT NULL,
    nickname VARCHAR(100),
    tracking_status VARCHAR(30) NOT NULL DEFAULT 'ACTIVE',
    is_removed BOOLEAN NOT NULL DEFAULT FALSE,
    monitoring_started_at TIMESTAMP,
    monitoring_paused_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, wallet_address)
);
-- =====================================================
-- INDEXED WALLETS
-- Global listener/indexer state
-- =====================================================
CREATE TABLE indexed_wallets (
    wallet_address VARCHAR(100) PRIMARY KEY,
    active_subscriber_count INTEGER NOT NULL DEFAULT 0,
    global_last_scanned_block BIGINT,
    latest_activity_block BIGINT,
    last_activity_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
-- =====================================================
-- WALLET ACTIVITIES
-- Global blockchain activity storage
-- =====================================================
CREATE TABLE wallet_activities (
    id UUID PRIMARY KEY,
    wallet_address VARCHAR(100) NOT NULL,
    network_id UUID NOT NULL REFERENCES networks(id),
    asset_deployment_id UUID REFERENCES asset_deployments(id),
    tx_hash VARCHAR(255) NOT NULL,
    block_number BIGINT NOT NULL,
    activity_type VARCHAR(20) NOT NULL,
    amount NUMERIC,
    metadata JSONB,
    occurred_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(tx_hash, wallet_address, network_id)
);
-- =====================================================
-- SUBSCRIPTIONS
-- Cached snapshot of onchain subscription state
-- Smart contract remains source of truth
-- =====================================================
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    plan_name VARCHAR(50) NOT NULL,
    subscription_status VARCHAR(30) NOT NULL,
    started_at TIMESTAMP,
    expires_at TIMESTAMP,
    contract_address VARCHAR(100),
    payment_tx_hash VARCHAR(255),
    updated_from_chain_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
-- =====================================================
-- PLAN FEATURES
-- Capability-driven pricing model
-- =====================================================
CREATE TABLE plan_features (
    plan_name VARCHAR(50) PRIMARY KEY,
    max_wallets INTEGER NOT NULL,
    backfill_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    retention_days INTEGER,
    realtime_monitoring BOOLEAN NOT NULL DEFAULT TRUE,
    alerts_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
-- =====================================================
-- BACKFILL JOBS
-- Historical replay queue for paid users
-- =====================================================
CREATE TABLE backfill_jobs (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    wallet_address VARCHAR(100) NOT NULL,
    from_block BIGINT NOT NULL,
    to_block BIGINT NOT NULL,
    network_id UUID NOT NULL REFERENCES networks(id),
    status VARCHAR(30) NOT NULL DEFAULT 'PENDING',
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);