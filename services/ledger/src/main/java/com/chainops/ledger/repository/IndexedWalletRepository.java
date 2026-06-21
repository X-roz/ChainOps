package com.chainops.ledger.repository;

import com.chainops.ledger.entity.IndexedWallet;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.Optional;
import java.util.UUID;

@Repository
public interface IndexedWalletRepository extends JpaRepository<IndexedWallet, UUID> {

    Optional<IndexedWallet> findByWalletAddressAndNetworkId(String walletAddress, UUID networkId);
}
