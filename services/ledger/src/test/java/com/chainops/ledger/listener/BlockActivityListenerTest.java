package com.chainops.ledger.listener;

import com.chainops.ledger.config.NatsProperties;
import com.chainops.ledger.schema.ActivityEvent;
import com.chainops.ledger.schema.BlockActivityMessage;
import com.chainops.ledger.service.WalletActivityService;
import com.fasterxml.jackson.core.JsonParseException;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.nats.client.Connection;
import io.nats.client.Dispatcher;
import io.nats.client.impl.Headers;
import io.nats.client.JetStream;
import io.nats.client.Message;
import io.nats.client.PushSubscribeOptions;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.test.util.ReflectionTestUtils;

import java.io.IOException;
import java.time.Instant;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class BlockActivityListenerTest {

    @Mock Connection nc;
    @Mock JetStream js;
    @Mock NatsProperties props;
    @Mock ObjectMapper mapper;
    @Mock WalletActivityService walletActivityService;
    @Mock Message msg;
    @Mock Dispatcher dispatcher;

    @InjectMocks BlockActivityListener listener;

    private final byte[] rawData = "{}".getBytes();
    private BlockActivityMessage sampleMessage;

    @BeforeEach
    void setUp() {
        ActivityEvent event = new ActivityEvent();
        event.setWalletAddress("0xWallet");
        event.setTxHash("0xTxHash");
        event.setEventType("NATIVE_TRANSFER");
        event.setActivityType("INCOMING");

        sampleMessage = new BlockActivityMessage();
        sampleMessage.setNetworkId("ethereum");
        sampleMessage.setBlockNumber(100L);
        sampleMessage.setBlockHash("0xBlockHash");
        sampleMessage.setBlockTimestamp(Instant.now());
        sampleMessage.setEvents(List.of(event));

    }

    // ── handle() ──────────────────────────────────────────────────────────────

    @Test
    void handle_validMessage_callsServiceAndAcks() throws Exception {
        when(mapper.readValue(rawData, BlockActivityMessage.class)).thenReturn(sampleMessage);
        when(msg.getData()).thenReturn(rawData);
        when(msg.getHeaders()).thenReturn(null);

        invokeHandle();

        verify(walletActivityService).persistAll(sampleMessage);
        verify(msg).ack();
        verify(msg, never()).nak();
    }

    @Test
    void handle_withBatchHeaders_logsHeadersAndAcks() throws Exception {
        Headers headers = mock(Headers.class);
        when(headers.getFirst("X-Batch-Index")).thenReturn("2");
        when(headers.getFirst("X-Total-Batches")).thenReturn("10");
        when(msg.getHeaders()).thenReturn(headers);
        when(msg.getData()).thenReturn(rawData);
        when(mapper.readValue(rawData, BlockActivityMessage.class)).thenReturn(sampleMessage);

        invokeHandle();

        verify(walletActivityService).persistAll(sampleMessage);
        verify(msg).ack();
        verify(msg, never()).nak();
    }

    @Test
    void handle_withNullHeaders_usesFallbackBatchValues() throws Exception {
        when(msg.getHeaders()).thenReturn(null);
        when(msg.getData()).thenReturn(rawData);
        when(mapper.readValue(rawData, BlockActivityMessage.class)).thenReturn(sampleMessage);

        invokeHandle();

        verify(msg).ack();
    }

    @Test
    void handle_nullEventsList_logsZeroEventsAndAcks() throws Exception {
        sampleMessage.setEvents(null);
        when(mapper.readValue(rawData, BlockActivityMessage.class)).thenReturn(sampleMessage);
        when(msg.getHeaders()).thenReturn(null);
        when(msg.getData()).thenReturn(rawData);
        when(msg.getData()).thenReturn(rawData);
        invokeHandle();

        verify(walletActivityService).persistAll(sampleMessage);
        verify(msg).ack();
        verify(msg, never()).nak();
    }

    @Test
    void handle_jsonParseException_naksWithoutCallingService() throws Exception {
        when(mapper.readValue(rawData, BlockActivityMessage.class))
                .thenThrow(new JsonParseException(null, "malformed json"));

        invokeHandle();

        verify(walletActivityService, never()).persistAll(any());
        verify(msg).nak();
        verify(msg, never()).ack();
    }

    @Test
    void handle_serviceThrowsRuntimeException_naksWithoutAck() throws Exception {
        when(mapper.readValue(rawData, BlockActivityMessage.class)).thenReturn(sampleMessage);
        when(msg.getHeaders()).thenReturn(null);
        doThrow(new RuntimeException("DB failure")).when(walletActivityService).persistAll(any());

        invokeHandle();

        verify(msg).nak();
        verify(msg, never()).ack();
    }

    @Test
    void handle_serviceThrowsIllegalArgument_naks() throws Exception {
        when(mapper.readValue(rawData, BlockActivityMessage.class)).thenReturn(sampleMessage);
        when(msg.getHeaders()).thenReturn(null);
        doThrow(new IllegalArgumentException("bad enum")).when(walletActivityService).persistAll(any());

        invokeHandle();

        verify(msg).nak();
        verify(msg, never()).ack();
    }

    // ── start() ───────────────────────────────────────────────────────────────

    @Test
    void start_successfulSubscription_setsRunningTrue() throws Exception {
        when(props.getConsumer()).thenReturn("ledger-consumer");
        when(props.getSubject()).thenReturn("chainops.block.activity");
        when(nc.createDispatcher()).thenReturn(dispatcher);

        listener.start();

        assertTrue(listener.isRunning());
        verify(js).subscribe(
                eq("chainops.block.activity"),
                eq(dispatcher),
                any(),
                eq(false),
                any(PushSubscribeOptions.class)
        );
    }

    @Test
    void start_subscribeThrows_throwsIllegalStateAndNotRunning() throws Exception {
        when(props.getConsumer()).thenReturn("ledger-consumer");
        when(props.getSubject()).thenReturn("chainops.block.activity");
        when(nc.createDispatcher()).thenReturn(dispatcher);
        when(js.subscribe(anyString(), any(), any(), anyBoolean(), any()))
                .thenThrow(new IOException("NATS unavailable"));

        assertThrows(IllegalStateException.class, () -> listener.start());
        assertFalse(listener.isRunning());
    }

    // ── stop() ────────────────────────────────────────────────────────────────

    @Test
    void stop_afterStart_closesDispatcherAndSetsNotRunning() throws Exception {
        when(props.getConsumer()).thenReturn("ledger-consumer");
        when(props.getSubject()).thenReturn("chainops.block.activity");
        when(nc.createDispatcher()).thenReturn(dispatcher);
        listener.start();

        listener.stop();

        assertFalse(listener.isRunning());
        verify(nc).closeDispatcher(dispatcher);
    }

    @Test
    void stop_dispatcherCloseThrows_doesNotPropagateException() throws Exception {
        when(props.getConsumer()).thenReturn("ledger-consumer");
        when(props.getSubject()).thenReturn("chainops.block.activity");
        when(nc.createDispatcher()).thenReturn(dispatcher);
        doThrow(new RuntimeException("close error")).when(nc).closeDispatcher(dispatcher);
        listener.start();

        assertDoesNotThrow(() -> listener.stop());
        assertFalse(listener.isRunning());
    }

    // ── isRunning() ───────────────────────────────────────────────────────────

    @Test
    void isRunning_beforeStart_returnsFalse() {
        assertFalse(listener.isRunning());
    }

    @Test
    void isRunning_afterStartAndStop_returnsFalse() throws Exception {
        when(props.getConsumer()).thenReturn("ledger-consumer");
        when(props.getSubject()).thenReturn("chainops.block.activity");
        when(nc.createDispatcher()).thenReturn(dispatcher);
        listener.start();
        assertThat(listener.isRunning()).isTrue();

        listener.stop();
        assertThat(listener.isRunning()).isFalse();
    }

    // ── helper ────────────────────────────────────────────────────────────────

    private void invokeHandle() {
        ReflectionTestUtils.invokeMethod(listener, "handle", msg);
    }
}
