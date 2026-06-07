package com.chainops.ledger.listener;

import com.chainops.ledger.config.NatsProperties;
import com.chainops.ledger.schema.BlockActivityMessage;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.nats.client.Connection;
import io.nats.client.Dispatcher;
import io.nats.client.JetStream;
import io.nats.client.Message;
import io.nats.client.api.AckPolicy;
import io.nats.client.api.ConsumerConfiguration;
import io.nats.client.api.DeliverPolicy;
import io.nats.client.PushSubscribeOptions;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.context.SmartLifecycle;
import org.springframework.stereotype.Component;

@Component
public class BlockActivityListener implements SmartLifecycle {

    private static final Logger log = LoggerFactory.getLogger(BlockActivityListener.class);

    private final Connection nc;
    private final JetStream js;
    private final NatsProperties props;
    private final ObjectMapper mapper;

    private Dispatcher dispatcher;
    private volatile boolean running = false;

    public BlockActivityListener(Connection nc, JetStream js, NatsProperties props, ObjectMapper mapper) {
        this.nc = nc;
        this.js = js;
        this.props = props;
        this.mapper = mapper;
    }

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
            BlockActivityMessage event = mapper.readValue(msg.getData(), BlockActivityMessage.class);

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
            log.error("Service = BlockActivityListener, failed to process message: {}", e.getMessage(), e);
            msg.nak();
        }
    }

    private void process(BlockActivityMessage msg) {
        // TODO: persist events to ledger storage
        if (msg.getEvents() == null) return;
        msg.getEvents().forEach(event ->
                log.debug("Service = BlockActivityListener, {} {} wallet={} tx={} amount={}",
                        event.getEventType(),
                        event.getActivityType(),
                        event.getWalletAddress(),
                        event.getTxHash(),
                        event.getAmount()));
    }
}
