package com.chainops.ledger.service;

import com.chainops.ledger.domain.response.SiweNonceResponse;
import lombok.extern.log4j.Log4j2;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;

import java.security.SecureRandom;
import java.time.Duration;
import java.time.Instant;
import java.util.HexFormat;
import java.util.concurrent.ConcurrentHashMap;

@Log4j2
@Service
public class SiweNonceService {

    private static final Duration NONCE_TTL = Duration.ofMinutes(5);
    private static final SecureRandom SECURE_RANDOM = new SecureRandom();

    private final ConcurrentHashMap<String, Instant> nonceStore = new ConcurrentHashMap<>();

    public record IssuedNonce(String nonce, Instant issuedAt, Instant expiresAt) {}

    public SiweNonceResponse issueNonce() {
        byte[] bytes = new byte[32];
        SECURE_RANDOM.nextBytes(bytes);
        String nonce = HexFormat.of().formatHex(bytes);
        Instant issuedAt = Instant.now();
        Instant expiresAt = issuedAt.plus(NONCE_TTL);
        nonceStore.put(nonce, expiresAt);
        log.info("Service = SiweNonceService, issued nonce expiresAt={}", expiresAt);
        return SiweNonceResponse.from(new IssuedNonce(nonce, issuedAt, expiresAt));
    }

    public boolean consumeNonce(String nonce) {
        Instant expiresAt = nonceStore.remove(nonce);
        if (expiresAt == null) {
            return false;
        }
        return Instant.now().isBefore(expiresAt);
    }

    @Scheduled(fixedRate = 60_000)
    public void evictExpired() {
        Instant now = Instant.now();
        nonceStore.entrySet().removeIf(entry -> entry.getValue().isBefore(now));
    }
}
