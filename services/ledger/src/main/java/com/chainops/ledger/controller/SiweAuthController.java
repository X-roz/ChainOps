package com.chainops.ledger.controller;

import com.chainops.ledger.schema.SiweNonceResponse;
import com.chainops.ledger.service.SiweNonceService;
import lombok.RequiredArgsConstructor;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1/auth/siwe")
@RequiredArgsConstructor
public class SiweAuthController {

    private final SiweNonceService siweNonceService;

    @PostMapping("/nonce")
    public ResponseEntity<SiweNonceResponse> issueNonce() {
        SiweNonceService.IssuedNonce issued = siweNonceService.issueNonce();
        return ResponseEntity.ok(new SiweNonceResponse(issued.nonce(), issued.issuedAt(), issued.expiresAt()));
    }
}
