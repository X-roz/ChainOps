ALTER TABLE wallet_activities
ADD CONSTRAINT uq_wallet_activity UNIQUE (tx_hash, indexed_wallet_id, activity_type);