package com.chainops.ledger.repository;

import com.chainops.ledger.entity.WalletActivity;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.UUID;

@Repository
public interface WalletActivityRepository extends JpaRepository<WalletActivity, UUID> {

    List<WalletActivity> findByIndexedWalletId(UUID indexedWalletId);

    List<WalletActivity> findByTxHash(String txHash);
}
