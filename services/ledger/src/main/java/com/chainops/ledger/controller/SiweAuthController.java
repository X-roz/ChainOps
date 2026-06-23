package com.chainops.ledger.controller;

import com.chainops.ledger.domain.response.ApiResponse;
import com.chainops.ledger.domain.response.SiweNonceResponse;
import com.chainops.ledger.service.SiweNonceService;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1/auth/siwe")
@RequiredArgsConstructor
public class SiweAuthController {

    private final SiweNonceService siweNonceService;

    @PostMapping("/nonce")
    public ApiResponse<SiweNonceResponse> issueNonce() {
        return ApiResponse.success(siweNonceService.issueNonce());
    }
}
