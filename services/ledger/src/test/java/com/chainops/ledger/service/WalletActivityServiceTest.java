package com.chainops.ledger.service;

import com.chainops.ledger.entity.ActivityType;
import com.chainops.ledger.entity.EventType;
import com.chainops.ledger.entity.IndexedWallet;
import com.chainops.ledger.entity.WalletActivity;
import com.chainops.ledger.repository.IndexedWalletRepository;
import com.chainops.ledger.repository.WalletActivityRepository;
import com.chainops.ledger.schema.ActivityEvent;
import com.chainops.ledger.schema.Asset;
import com.chainops.ledger.schema.BlockActivityMessage;
import com.chainops.ledger.schema.GasDetails;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.dao.DataIntegrityViolationException;

import java.math.BigDecimal;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.time.LocalDateTime;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.lenient;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class WalletActivityServiceTest {

    @Mock WalletActivityRepository repository;
    @Mock IndexedWalletRepository indexedWalletRepository;

    @InjectMocks WalletActivityService service;

    private BlockActivityMessage message;
    private ActivityEvent event;
    private UUID networkId;

    @BeforeEach
    void setUp() {
        networkId = UUID.randomUUID();

        event = new ActivityEvent();
        event.setWalletAddress("0xWallet123");
        event.setTxHash("0xTxHash456");
        event.setEventType("NATIVE_TRANSFER");
        event.setActivityType("INCOMING");
        event.setFromAddress("0xFrom");
        event.setToAddress("0xTo");
        event.setAmount("1.5");

        message = new BlockActivityMessage();
        message.setNetworkId(networkId.toString());
        message.setBlockNumber(200L);
        message.setBlockHash("0xBlockHash");
        message.setBlockTimestamp(Instant.parse("2024-01-15T10:30:00Z"));
        message.setEvents(List.of(event));

        // default: every wallet/network pair resolves to a deterministic indexed wallet
        lenient().when(indexedWalletRepository.findByWalletAddressAndNetworkId(anyString(), any(UUID.class)))
                .thenAnswer(invocation -> {
                    String walletAddress = invocation.getArgument(0);
                    IndexedWallet indexedWallet = new IndexedWallet();
                    indexedWallet.setId(UUID.nameUUIDFromBytes(walletAddress.getBytes(StandardCharsets.UTF_8)));
                    indexedWallet.setWalletAddress(walletAddress);
                    indexedWallet.setNetworkId(invocation.getArgument(1));
                    return Optional.of(indexedWallet);
                });
    }

    // ── skip cases ────────────────────────────────────────────────────────────

    @Test
    void persistAll_nullEvents_skipsWithoutSaving() {
        message.setEvents(null);

        service.persistAll(message);

        verify(repository, never()).saveAll(anyList());
    }

    @Test
    void persistAll_emptyEvents_skipsWithoutSaving() {
        message.setEvents(Collections.emptyList());

        service.persistAll(message);

        verify(repository, never()).saveAll(anyList());
    }

    // ── happy path ────────────────────────────────────────────────────────────

    @Test
    void persistAll_validEvent_savesCorrectlyMappedActivity() {
        service.persistAll(message);

        WalletActivity saved = captureFirst();
        assertThat(saved.getTxHash()).isEqualTo("0xTxHash456");
        assertThat(saved.getBlockNumber()).isEqualTo(200L);
        assertThat(saved.getEventType()).isEqualTo(EventType.NATIVE_TRANSFER);
        assertThat(saved.getActivityType()).isEqualTo(ActivityType.INCOMING);
        assertThat(saved.getFromAddress()).isEqualTo("0xFrom");
        assertThat(saved.getToAddress()).isEqualTo("0xTo");
        assertThat(saved.getAmount()).isEqualByComparingTo(new BigDecimal("1.5"));
    }

    @Test
    void persistAll_multipleEvents_allSaved() {
        ActivityEvent second = buildEvent("0xWallet999", "0xTx999", "TOKEN_TRANSFER", "OUTGOING");
        message.setEvents(List.of(event, second));

        service.persistAll(message);

        assertThat(captureAll()).hasSize(2);
    }

    @Test
    void persistAll_blockTimestamp_convertedToUtcLocalDateTime() {
        message.setBlockTimestamp(Instant.parse("2024-06-15T12:30:00Z"));

        service.persistAll(message);

        LocalDateTime ts = captureFirst().getBlockTimestamp();
        assertThat(ts.getYear()).isEqualTo(2024);
        assertThat(ts.getMonthValue()).isEqualTo(6);
        assertThat(ts.getDayOfMonth()).isEqualTo(15);
        assertThat(ts.getHour()).isEqualTo(12);
        assertThat(ts.getMinute()).isEqualTo(30);
    }

    // ── wallet UUID resolution ───────────────────────────────────────────────

    @Test
    void persistAll_sameWalletAddress_producesSameIndexedWalletId() {
        ActivityEvent second = buildEvent("0xWallet123", "0xTx999", "TOKEN_TRANSFER", "OUTGOING");
        message.setEvents(List.of(event, second));

        service.persistAll(message);

        List<WalletActivity> saved = captureAll();
        assertThat(saved.get(0).getIndexedWalletId()).isEqualTo(saved.get(1).getIndexedWalletId());
    }

    @Test
    void persistAll_differentWalletAddresses_produceDifferentIndexedWalletIds() {
        ActivityEvent second = buildEvent("0xDifferentWallet", "0xTx999", "TOKEN_TRANSFER", "OUTGOING");
        message.setEvents(List.of(event, second));

        service.persistAll(message);

        List<WalletActivity> saved = captureAll();
        assertThat(saved.get(0).getIndexedWalletId()).isNotEqualTo(saved.get(1).getIndexedWalletId());
    }

    @Test
    void persistAll_walletAddress_indexedWalletIdIsNotNull() {
        service.persistAll(message);

        UUID id = captureFirst().getIndexedWalletId();
        assertThat(id).isNotNull();
    }

    @Test
    void persistAll_unknownIndexedWallet_skipsEventAndSavesEmptyList() {
        when(indexedWalletRepository.findByWalletAddressAndNetworkId(event.getWalletAddress(), networkId))
                .thenReturn(Optional.empty());

        service.persistAll(message);

        assertThat(captureAll()).isEmpty();
    }

    // ── amount ────────────────────────────────────────────────────────────────

    @Test
    void persistAll_nullAmount_savedWithNullAmount() {
        event.setAmount(null);

        service.persistAll(message);

        assertThat(captureFirst().getAmount()).isNull();
    }

    @Test
    void persistAll_invalidAmount_skipsEventAndSavesEmptyList() {
        event.setAmount("not-a-number");

        service.persistAll(message);

        assertThat(captureAll()).isEmpty();
    }

    // ── asset ─────────────────────────────────────────────────────────────────

    @Test
    void persistAll_withAsset_mapsAssetFields() {
        Asset asset = new Asset();
        asset.setAssetType("ERC20");
        asset.setSymbol("USDT");
        asset.setContractAddress("0xContractAddr");
        event.setAsset(asset);

        service.persistAll(message);

        WalletActivity saved = captureFirst();
        assertThat(saved.getAssetType()).isEqualTo("ERC20");
        assertThat(saved.getAssetSymbol()).isEqualTo("USDT");
        assertThat(saved.getAssetContractAddress()).isEqualTo("0xContractAddr");
    }

    @Test
    void persistAll_nullAsset_assetFieldsAreNull() {
        event.setAsset(null);

        service.persistAll(message);

        WalletActivity saved = captureFirst();
        assertThat(saved.getAssetType()).isNull();
        assertThat(saved.getAssetSymbol()).isNull();
        assertThat(saved.getAssetContractAddress()).isNull();
    }

    // ── gas ───────────────────────────────────────────────────────────────────

    @Test
    void persistAll_withGas_mapsFeeFields() {
        GasDetails gas = new GasDetails();
        gas.setFeePaid("0.001");
        gas.setFeeAsset("ETH");
        gas.setGasUsed(21000L);
        gas.setEffectiveGasPrice("50000000000");
        event.setGas(gas);

        service.persistAll(message);

        WalletActivity saved = captureFirst();
        assertThat(saved.getFeePaid()).isEqualByComparingTo(new BigDecimal("0.001"));
        assertThat(saved.getFeeAsset()).isEqualTo("ETH");
    }

    @Test
    void persistAll_nullGas_feeFieldsAreNull() {
        event.setGas(null);

        service.persistAll(message);

        WalletActivity saved = captureFirst();
        assertThat(saved.getFeePaid()).isNull();
        assertThat(saved.getFeeAsset()).isNull();
    }

    @Test
    void persistAll_gasWithNullFeePaid_feePaidIsNull() {
        GasDetails gas = new GasDetails();
        gas.setFeePaid(null);
        gas.setFeeAsset("ETH");
        event.setGas(gas);

        service.persistAll(message);

        assertThat(captureFirst().getFeePaid()).isNull();
    }

    // ── metadata ──────────────────────────────────────────────────────────────

    @Test
    void persistAll_withMetadata_metadataMapped() {
        event.setMetadata(Map.of("key", "value", "count", 5));

        service.persistAll(message);

        assertThat(captureFirst().getMetadata()).containsEntry("key", "value");
    }

    @Test
    void persistAll_nullMetadata_metadataIsNull() {
        event.setMetadata(null);

        service.persistAll(message);

        assertThat(captureFirst().getMetadata()).isNull();
    }

    // ── enum validation ───────────────────────────────────────────────────────

    @Test
    void persistAll_invalidEventType_skipsEventAndSavesEmptyList() {
        event.setEventType("UNKNOWN_EVENT");

        service.persistAll(message);

        assertThat(captureAll()).isEmpty();
    }

    @Test
    void persistAll_invalidActivityType_skipsEventAndSavesEmptyList() {
        event.setActivityType("UNKNOWN_ACTIVITY");

        service.persistAll(message);

        assertThat(captureAll()).isEmpty();
    }

    @Test
    void persistAll_allValidEventTypes_saveSuccessfully() {
        for (EventType type : EventType.values()) {
            event.setEventType(type.name());
            service.persistAll(message);
        }

        verify(repository, times(EventType.values().length)).saveAll(anyList());
    }

    @Test
    void persistAll_allValidActivityTypes_saveSuccessfully() {
        for (ActivityType type : ActivityType.values()) {
            event.setActivityType(type.name());
            service.persistAll(message);
        }

        verify(repository, times(ActivityType.values().length)).saveAll(anyList());
    }

    // ── partial batch failure (skip-and-continue) ────────────────────────────

    @Test
    void persistAll_oneInvalidEventAmongMany_skipsBadEventAndSavesTheRest() {
        ActivityEvent bad = buildEvent("0xBadWallet", "0xTxBad", "UNKNOWN_EVENT", "INCOMING");
        ActivityEvent good = buildEvent("0xGoodWallet", "0xTxGood", "TOKEN_TRANSFER", "OUTGOING");
        message.setEvents(List.of(event, bad, good));

        service.persistAll(message);

        List<WalletActivity> saved = captureAll();
        assertThat(saved).extracting(WalletActivity::getTxHash)
                .containsExactly("0xTxHash456", "0xTxGood");
    }

    @Test
    void persistAll_allEventsInvalid_savesEmptyListWithoutThrowing() {
        event.setEventType("UNKNOWN_EVENT");

        service.persistAll(message);

        verify(repository).saveAll(Collections.emptyList());
    }

    // ── message-level failure ─────────────────────────────────────────────────

    @Test
    void persistAll_invalidNetworkId_throwsAndDoesNotSave() {
        message.setNetworkId("not-a-uuid");

        assertThatThrownBy(() -> service.persistAll(message))
                .isInstanceOf(IllegalArgumentException.class);
        verify(repository, never()).saveAll(anyList());
    }

    // ── DB error ──────────────────────────────────────────────────────────────

    @Test
    void persistAll_dataAccessException_logsAndRethrows() {
        DataIntegrityViolationException dbEx = new DataIntegrityViolationException("constraint violation");
        when(repository.saveAll(anyList())).thenThrow(dbEx);

        assertThatThrownBy(() -> service.persistAll(message))
                .isSameAs(dbEx);
    }

    @Test
    void persistAll_genericRepositoryException_rethrows() {
        RuntimeException ex = new RuntimeException("unexpected");
        when(repository.saveAll(anyList())).thenThrow(ex);

        assertThatThrownBy(() -> service.persistAll(message))
                .isSameAs(ex);
    }

    // ── helpers ───────────────────────────────────────────────────────────────

    @SuppressWarnings("unchecked")
    private List<WalletActivity> captureAll() {
        ArgumentCaptor<List<WalletActivity>> captor = ArgumentCaptor.forClass(List.class);
        verify(repository).saveAll(captor.capture());
        return captor.getValue();
    }

    private WalletActivity captureFirst() {
        return captureAll().get(0);
    }

    private ActivityEvent buildEvent(String wallet, String tx, String eventType, String activityType) {
        ActivityEvent e = new ActivityEvent();
        e.setWalletAddress(wallet);
        e.setTxHash(tx);
        e.setEventType(eventType);
        e.setActivityType(activityType);
        return e;
    }
}
