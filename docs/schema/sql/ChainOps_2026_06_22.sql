ALTER TABLE wallet_activities
ADD CONSTRAINT uq_wallet_activity UNIQUE (tx_hash, indexed_wallet_id, activity_type);
-- users: who has an account
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_status VARCHAR(30) NOT NULL DEFAULT 'ACTIVE',
    current_plan VARCHAR(50) NOT NULL DEFAULT 'FREE_TRIAL',
    trial_expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMP
);
-- auth_wallets: which wallets can sign in. V1: one per user.
CREATE TABLE auth_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wallet_address VARCHAR(100) NOT NULL UNIQUE,
    verified_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
-- user_tracked_wallets: which wallets a user monitors. V1: one per user, same as auth wallet.
CREATE TABLE user_tracked_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    indexed_wallet_id UUID NOT NULL REFERENCES indexed_wallets(id),
    label VARCHAR(100),
    -- free-text user label: "Revenue", "Payroll", "Recurring", "Personal", etc.
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, indexed_wallet_id)
);