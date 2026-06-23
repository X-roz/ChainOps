package com.chainops.ledger.domain.listener;

import com.fasterxml.jackson.annotation.JsonProperty;
import lombok.Data;

import java.time.Instant;
import java.util.List;

@Data
public class BlockActivityMessage {

    @JsonProperty("network_id")
    private String networkId;

    @JsonProperty("block_number")
    private long blockNumber;

    @JsonProperty("block_hash")
    private String blockHash;

    @JsonProperty("block_timestamp")
    private Instant blockTimestamp;

    @JsonProperty("events")
    private List<ActivityEvent> events;

}
