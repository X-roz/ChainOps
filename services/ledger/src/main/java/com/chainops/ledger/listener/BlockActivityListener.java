package com.chainops.ledger.listener;

import com.chainops.ledger.config.NatsProperties;
import com.chainops.ledger.domain.listener.BlockActivityMessage;
import com.chainops.ledger.service.WalletActivityService;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.nats.client.Connection;
import io.nats.client.Dispatcher;
import io.nats.client.JetStream;
import io.nats.client.Message;
import io.nats.client.api.AckPolicy;
import io.nats.client.api.ConsumerConfiguration;
import io.nats.client.api.DeliverPolicy;
import io.nats.client.PushSubscribeOptions;
import lombok.RequiredArgsConstructor;
import lombok.extern.log4j.Log4j2;
import org.springframework.context.SmartLifecycle;
import org.springframework.stereotype.Component;

@Log4j2
@Component
@RequiredArgsConstructor
public class BlockActivityListener implements SmartLifecycle {

    private final Connection nc;
    private final JetStream js;
    private final NatsProperties props;
    private final ObjectMapper mapper;
    private final WalletActivityService walletActivityService;

    private Dispatcher dispatcher;
    private volatile boolean running = false;

    @Override
    public void start() {
        try {
            ConsumerConfiguration cc = ConsumerConfiguration.builder()
                    .durable(props.getConsumer())
                    .ackPolicy(AckPolicy.Explicit)
                    .deliverPolicy(DeliverPolicy.New)
                    .build();

            PushSubscribeOptions subscribeOptions = PushSubscribeOptions.builder()
                    .configuration(cc)
                    .build();

            dispatcher = nc.createDispatcher();
            js.subscribe(props.getSubject(), dispatcher, this::handle, false, subscribeOptions);

            running = true;
            log.info("Service = BlockActivityListener, subscribed to {} as consumer '{}'", props.getSubject(), props.getConsumer());
        } catch (Exception e) {
            throw new IllegalStateException("Service = BlockActivityListener, Failed to subscribe to NATS subject: " + props.getSubject(), e);
        }
    }

    @Override
    public void stop() {
        running = false;
        if (dispatcher != null) {
            try {
                nc.closeDispatcher(dispatcher);
            } catch (Exception e) {
                log.warn("Service = BlockActivityListener, error closing dispatcher: {}", e.getMessage());
            }
        }
        log.info("Service = BlockActivityListener, listener stopped");
    }

    @Override
    public boolean isRunning() {
        return running;
    }

    private void handle(Message msg) {
        try {
            BlockActivityMessage event;
            event = mapper.readValue(msg.getData(), BlockActivityMessage.class);

            String batchIndex = msg.getHeaders() != null ? msg.getHeaders().getFirst("X-Batch-Index") : "?";
            String totalBatches = msg.getHeaders() != null ? msg.getHeaders().getFirst("X-Total-Batches") : "?";

            log.info("Service = BlockActivityListener, block={} network={} events={} batch={}/{}",
                    event.getBlockNumber(),
                    event.getNetworkId(),
                    event.getEvents() == null ? 0 : event.getEvents().size(),
                    batchIndex,
                    totalBatches);
            process(event);
            msg.ack();
        } catch (Exception e) {
            log.error("Service = BlockActivityListener, failed to persist : {}", e.getMessage(), e);
            msg.nak();
        }
    }

    private void process(BlockActivityMessage msg) {
        walletActivityService.persistAll(msg);
    }
}
