package com.chainops.ledger.service;

import com.chainops.ledger.entity.ActivityType;
import com.chainops.ledger.entity.EventType;
import com.chainops.ledger.entity.WalletActivity;
import com.chainops.ledger.repository.WalletActivityRepository;
import com.chainops.ledger.schema.ActivityEvent;
import com.chainops.ledger.schema.BlockActivityMessage;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.math.BigDecimal;
import java.nio.charset.StandardCharsets;
import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.util.List;
import java.util.UUID;

@Service
public class WalletActivityService {

    private static final Logger log = LoggerFactory.getLogger(WalletActivityService.class);

    private final WalletActivityRepository repository;

    public WalletActivityService(WalletActivityRepository repository) {
        this.repository = repository;
    }

    @Transactional
    public void persistAll(BlockActivityMessage message) {
        if (message.getEvents() == null || message.getEvents().isEmpty()) return;

        LocalDateTime blockTimestamp = LocalDateTime.ofInstant(message.getBlockTimestamp(), ZoneOffset.UTC);

        List<WalletActivity> activities = message.getEvents().stream()
                .map(event -> toEntity(event, message.getBlockNumber(), blockTimestamp))
                .toList();

        repository.saveAll(activities);

        log.info("Service = WalletActivityService, persisted {} activities for block={}", activities.size(), message.getBlockNumber());
    }

    private WalletActivity toEntity(ActivityEvent event, long blockNumber, LocalDateTime blockTimestamp) {
        WalletActivity activity = new WalletActivity();

        // Deterministic UUID from wallet address until an indexed_wallets lookup is in place
        activity.setIndexedWalletId(UUID.nameUUIDFromBytes(event.getWalletAddress().getBytes(StandardCharsets.UTF_8)));
        activity.setTxHash(event.getTxHash());
        activity.setBlockNumber(blockNumber);
        activity.setBlockTimestamp(blockTimestamp);
        activity.setEventType(EventType.valueOf(event.getEventType()));
        activity.setActivityType(ActivityType.valueOf(event.getActivityType()));
        activity.setFromAddress(event.getFromAddress());
        activity.setToAddress(event.getToAddress());
        activity.setAmount(event.getAmount() != null ? new BigDecimal(event.getAmount()) : null);
        activity.setMetadata(event.getMetadata());

        if (event.getAsset() != null) {
            activity.setAssetType(event.getAsset().getAssetType());
            activity.setAssetSymbol(event.getAsset().getSymbol());
            activity.setAssetContractAddress(event.getAsset().getContractAddress());
        }

        if (event.getGas() != null) {
            activity.setFeePaid(event.getGas().getFeePaid() != null ? new BigDecimal(event.getGas().getFeePaid()) : null);
            activity.setFeeAsset(event.getGas().getFeeAsset());
        }

        return activity;
    }
}
