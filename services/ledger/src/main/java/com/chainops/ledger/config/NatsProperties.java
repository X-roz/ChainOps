package com.chainops.ledger.config;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;

@Data
@ConfigurationProperties(prefix = "nats")
public class NatsProperties {

    private String url;
    private String subject;
    private String consumer;
}
