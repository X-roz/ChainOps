package com.chainops.ledger.domain.listener;

import com.fasterxml.jackson.annotation.JsonProperty;
import lombok.Data;

@Data
public class GasDetails {

    @JsonProperty("fee_paid")
    private String feePaid;

    @JsonProperty("fee_asset")
    private String feeAsset;

    @JsonProperty("gas_used")
    private long gasUsed;

    @JsonProperty("effective_gas_price")
    private String effectiveGasPrice;

}
