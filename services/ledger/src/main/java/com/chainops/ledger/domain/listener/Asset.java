package com.chainops.ledger.domain.listener;

import com.fasterxml.jackson.annotation.JsonProperty;
import lombok.Data;

@Data
public class Asset {

    @JsonProperty("type")
    private String assetType;

    @JsonProperty("symbol")
    private String symbol;

    @JsonProperty("contract_address")
    private String contractAddress;

}
