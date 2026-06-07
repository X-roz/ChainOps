package com.chainops.ledger.config;

import io.nats.client.Connection;
import io.nats.client.JetStream;
import io.nats.client.Nats;
import io.nats.client.Options;
import lombok.extern.log4j.Log4j2;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

import java.time.Duration;

@Log4j2
@Configuration
@EnableConfigurationProperties(NatsProperties.class)
public class NatsConfig {

    @Bean
    public Connection natsConnection(NatsProperties props) throws Exception {
        Options options = new Options.Builder()
                .server(props.getUrl())
                .maxReconnects(-1)
                .reconnectWait(Duration.ofSeconds(2))
                .connectionListener((conn, type) -> log.info("service = natsConnection, connection event: {}", type))
                .build();
        return Nats.connect(options);
    }

    @Bean
    public JetStream jetStream(Connection nc) throws Exception {
        return nc.jetStream();
    }
}
