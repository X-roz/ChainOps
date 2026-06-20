package com.chainops.ledger.service;

import com.chainops.ledger.entity.ActivityType;
import com.chainops.ledger.entity.EventType;
import com.chainops.ledger.entity.WalletActivity;
import com.chainops.ledger.repository.WalletActivityRepository;
import com.chainops.ledger.schema.ActivityEvent;
import com.chainops.ledger.schema.BlockActivityMessage;
import lombok.extern.log4j.Log4j2;
import org.springframework.dao.DataAccessException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.math.BigDecimal;
import java.nio.charset.StandardCharsets;
import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.util.List;
import java.util.UUID;

@Log4j2
@Service
public class WalletActivityService {

    private final WalletActivityRepository repository;

    public WalletActivityService(WalletActivityRepository repository) {
        this.repository = repository;
    }

    @Transactional
    public void persistAll(BlockActivityMessage message) {
        log.info("Service = WalletActivityService, persistAll started block={} network={} events={}",
                message.getBlockNumber(), message.getNetworkId(),
                message.getEvents() == null ? 0 : message.getEvents().size());

        if (message.getEvents() == null || message.getEvents().isEmpty()) {
            log.info("Service = WalletActivityService, persistAll skipped - no events block={} network={}",
                    message.getBlockNumber(), message.getNetworkId());
            return;
        }

        try {
            LocalDateTime blockTimestamp = LocalDateTime.ofInstant(message.getBlockTimestamp(), ZoneOffset.UTC);
            List<WalletActivity> activities;
            activities = message.getEvents().stream()
                    .map(event -> toEntity(event, message.getBlockNumber(), blockTimestamp))
                    .toList();
            repository.saveAll(activities);
            log.info("Service = WalletActivityService, persisted {} activities for block={} network={}",
                    activities.size(), message.getBlockNumber(), message.getNetworkId());
        } catch (DataAccessException e) {
            log.error("Service = WalletActivityService, DB error persisting block={} network={}: {}",
                    message.getBlockNumber(), message.getNetworkId(), e.getMessage(), e);
            throw e;
        } catch (Exception e) {
            log.error("Service = WalletActivityService, failed to map events block={} network={}: {}",
                    message.getBlockNumber(), message.getNetworkId(), e.getMessage(), e);
            throw e;
        }
    }

    private WalletActivity toEntity(ActivityEvent event, long blockNumber, LocalDateTime blockTimestamp) {
        try {
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
        } catch (IllegalArgumentException e) {
            log.error("Service = WalletActivityService, invalid enum or number value wallet={} tx={} block={}: {}",
                    event.getWalletAddress(), event.getTxHash(), blockNumber, e.getMessage(), e);
            throw e;
        } catch (Exception e) {
            log.error("Service = WalletActivityService, failed to map event wallet={} tx={} block={}: {}",
                    event.getWalletAddress(), event.getTxHash(), blockNumber, e.getMessage(), e);
            throw e;
        }
    }
}
