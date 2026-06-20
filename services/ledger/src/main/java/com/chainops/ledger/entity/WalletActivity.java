package com.chainops.ledger.entity;

import jakarta.persistence.*;
import lombok.Data;
import org.hibernate.annotations.JdbcTypeCode;
import org.hibernate.type.SqlTypes;

import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.Map;
import java.util.UUID;

@Data
@Entity
@Table(name = "wallet_activities")
public class WalletActivity {

    @Id
    @GeneratedValue(strategy = GenerationType.UUID)
    @Column(columnDefinition = "uuid", updatable = false, nullable = false)
    private UUID id;

    @Column(name = "indexed_wallet_id", nullable = false)
    private UUID indexedWalletId;

    @Column(name = "tx_hash", nullable = false, length = 100)
    private String txHash;

    @Column(name = "block_number", nullable = false)
    private Long blockNumber;

    @Column(name = "block_timestamp", nullable = false)
    private LocalDateTime blockTimestamp;

    @Enumerated(EnumType.STRING)
    @Column(name = "event_type", nullable = false, length = 50)
    private EventType eventType;

    @Enumerated(EnumType.STRING)
    @Column(name = "activity_type", nullable = false, length = 50)
    private ActivityType activityType;

    @Column(name = "from_address", length = 100)
    private String fromAddress;

    @Column(name = "to_address", length = 100)
    private String toAddress;

    @Column(name = "amount", precision = 38, scale = 18)
    private BigDecimal amount;

    @Column(name = "asset_type", length = 50)
    private String assetType;

    @Column(name = "asset_symbol", length = 50)
    private String assetSymbol;

    @Column(name = "asset_contract_address", length = 100)
    private String assetContractAddress;

    @Column(name = "fee_paid", precision = 38, scale = 18)
    private BigDecimal feePaid;

    @Column(name = "fee_asset", length = 20)
    private String feeAsset;

    @JdbcTypeCode(SqlTypes.JSON)
    @Column(columnDefinition = "jsonb")
    private Map<String, Object> metadata;

    @Column(name = "created_at", nullable = false, updatable = false)
    private LocalDateTime createdAt;

    @PrePersist
    protected void onCreate() {
        createdAt = LocalDateTime.now();
    }
}
