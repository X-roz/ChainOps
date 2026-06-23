package com.chainops.ledger.domain.response;

import com.chainops.ledger.service.SiweNonceService;
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

    public static SiweNonceResponse from(SiweNonceService.IssuedNonce issued) {
        return new SiweNonceResponse(issued.nonce(), issued.issuedAt(), issued.expiresAt());
    }
}
