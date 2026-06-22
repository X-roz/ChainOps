package com.chainops.ledger.schema;

import com.fasterxml.jackson.annotation.JsonProperty;
import lombok.AllArgsConstructor;
import lombok.Getter;

import java.time.Instant;

@Getter
@AllArgsConstructor
public class SiweNonceResponse {

    private final String nonce;

    @JsonProperty("issued_at")
    private final Instant issuedAt;

    @JsonProperty("expires_at")
    private final Instant expiresAt;
}
