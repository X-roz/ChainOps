package com.chainops.ledger.domain.listener;

import com.fasterxml.jackson.annotation.JsonProperty;
import lombok.Data;

import java.util.Map;

@Data
public class ActivityEvent {

    @JsonProperty("wallet_address")
    private String walletAddress;

    @JsonProperty("tx_hash")
    private String txHash;

    @JsonProperty("event_type")
    private String eventType;

    @JsonProperty("activity_type")
    private String activityType;

    @JsonProperty("from_address")
    private String fromAddress;

    @JsonProperty("to_address")
    private String toAddress;

    @JsonProperty("amount")
    private String amount;

    @JsonProperty("asset")
    private Asset asset;

    @JsonProperty("gas")
    private GasDetails gas;

    @JsonProperty("metadata")
    private Map<String, Object> metadata;
}
