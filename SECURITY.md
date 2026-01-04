# 🔐 Security Architecture

This document explains the security design, cryptographic implementation, and threat model of the Secure P2P File Vault.

## Table of Contents

1. [Overview](#overview)
2. [Cryptographic Design](#cryptographic-design)
3. [Key Exchange Protocol](#key-exchange-protocol)
4. [File Encryption Process](#file-encryption-process)
5. [Integrity Verification](#integrity-verification)
6. [Threat Model](#threat-model)
7. [Security Guarantees](#security-guarantees)
8. [Known Limitations](#known-limitations)

---

## Overview

The Secure P2P File Vault implements end-to-end encryption for peer-to-peer file transfers using WebRTC DataChannels. The system is designed so that:

- **The server never sees plaintext file data or encryption keys**
- **Files are encrypted before transmission**
- **Keys are ephemeral and never persist**
- **Integrity is cryptographically verified**

## Cryptographic Design

### Encryption Algorithm: AES-GCM

We use **AES-256-GCM** (Advanced Encryption Standard with Galois/Counter Mode) for file encryption.

**Why AES-GCM?**
- ✅ **Authenticated Encryption**: Provides both confidentiality and integrity
- ✅ **Performance**: Hardware-accelerated on most modern CPUs
- ✅ **Standard**: NIST-approved, widely reviewed
- ✅ **Browser Support**: Native implementation via Web Crypto API
- ✅ **Streaming**: Suitable for chunked encryption

**Parameters:**
```
Algorithm: AES-GCM
Key Size: 256 bits
IV Size: 96 bits (12 bytes)
Tag Size: 128 bits (16 bytes, built into GCM)
```

### Hash Function: SHA-256

Used for integrity verification of the complete file.

**Why SHA-256?**
- ✅ **Collision Resistant**: Computationally infeasible to find collisions
- ✅ **Preimage Resistant**: Cannot reverse hash to find original data
- ✅ **Standard**: Part of SHA-2 family, widely trusted
- ✅ **Browser Support**: Native implementation via Web Crypto API

---

## Key Exchange Protocol

### Phase 1: Session Initialization

```
1. Sender generates session
   ├─ Server creates unique sessionId
   └─ Sender receives sessionId

2. Sender generates encryption key
   ├─ AES-GCM 256-bit key (crypto.subtle.generateKey)
   ├─ Random IV (96 bits via crypto.getRandomValues)
   └─ Keys stored in browser memory only
```

### Phase 2: Peer Connection

```
3. Receiver joins with sessionId
   ├─ Server matches peers
   └─ WebRTC signaling begins

4. WebRTC DataChannel established
   ├─ DTLS encryption for transport (mandatory in WebRTC)
   ├─ Perfect Forward Secrecy via DTLS
   └─ Secure channel for key transmission
```

### Phase 3: Key Transmission

```
5. Sender sends metadata over DataChannel
   {
     "type": "metadata",
     "data": {
       "encryptionKey": [raw key bytes],
       "iv": [IV bytes],
       "hash": "sha256-hash",
       ...
     }
   }

6. Receiver imports encryption key
   └─ crypto.subtle.importKey(keyData, 'AES-GCM')
```

**Security Note:** The encryption key is transmitted over the DTLS-encrypted DataChannel, which provides transport-layer security. The signaling server never sees this key exchange.

---

## File Encryption Process

### Chunking Strategy

Files are split into **64KB chunks** for efficient transmission and progress tracking.

```javascript
CHUNK_SIZE = 64 * 1024  // 64KB
totalChunks = ceil(fileSize / CHUNK_SIZE)
```

### Per-Chunk Encryption

Each chunk is encrypted independently with a unique IV:

```javascript
// Base IV (random 96 bits)
baseIV = crypto.getRandomValues(new Uint8Array(12))

// Per-chunk IV derivation
chunkIV = baseIV XOR chunkIndex

// Encryption
encryptedChunk = AES-GCM-Encrypt(
    key: encryptionKey,
    iv: chunkIV,
    plaintext: chunk
)
```

**Why unique IVs per chunk?**
- ✅ Prevents IV reuse (security requirement for GCM)
- ✅ Simple deterministic derivation
- ✅ No need to transmit per-chunk IVs

### Encryption Flow Diagram

```
Original File (e.g., 200KB)
    ↓
Split into chunks
    ↓
┌──────────────┬──────────────┬──────────────┐
│  Chunk 0     │  Chunk 1     │  Chunk 2     │
│  (64KB)      │  (64KB)      │  (72KB)      │
└──────────────┴──────────────┴──────────────┘
    ↓                ↓                ↓
Encrypt with      Encrypt with     Encrypt with
IV = base XOR 0   IV = base XOR 1  IV = base XOR 2
    ↓                ↓                ↓
┌──────────────┬──────────────┬──────────────┐
│ Encrypted 0  │ Encrypted 1  │ Encrypted 2  │
│ (+ 16B tag)  │ (+ 16B tag)  │ (+ 16B tag)  │
└──────────────┴──────────────┴──────────────┘
    ↓                ↓                ↓
    Send via WebRTC DataChannel
```

### Decryption Process

Receiver decrypts chunks in the same order:

```javascript
for (let i = 0; i < totalChunks; i++) {
    chunkIV = baseIV XOR i
    decryptedChunk = AES-GCM-Decrypt(
        key: encryptionKey,
        iv: chunkIV,
        ciphertext: encryptedChunks[i]
    )
}

// Reassemble file
completeFile = concat(decryptedChunks)
```

---

## Integrity Verification

### Hash Calculation (Sender)

```javascript
// Before encryption
originalFile = readFile()
hash = SHA-256(originalFile)

// Transmitted in metadata
metadata = {
    hash: hexEncode(hash),
    ...
}
```

### Hash Verification (Receiver)

```javascript
// After decryption and reassembly
receivedFile = reassemble(decryptedChunks)
receivedHash = SHA-256(receivedFile)

// Verify
if (receivedHash === metadata.hash) {
    ✅ File integrity confirmed
} else {
    ❌ File corrupted or tampered
}
```

**Why SHA-256 on complete file?**
- Detects any corruption during transmission
- Detects tampering attempts
- Verifies decryption succeeded correctly
- Simple, single verification step

---

## Threat Model

### Adversary Capabilities

We consider adversaries with the following capabilities:

1. **Passive Network Eavesdropper**
   - Can observe all network traffic
   - Cannot modify packets

2. **Malicious Signaling Server**
   - Controls the signaling server
   - Can see WebSocket messages
   - Cannot break DTLS encryption

3. **Active Network Attacker (Limited)**
   - Can drop or delay packets
   - Cannot break WebRTC DTLS

### Attack Scenarios & Defenses

#### 1. Eavesdropping on File Content

**Attack:** Adversary intercepts encrypted file chunks

**Defense:**
- ✅ AES-GCM encryption with 256-bit keys
- ✅ Keys transmitted via DTLS-encrypted DataChannel
- ✅ Server never sees keys or plaintext

**Result:** ❌ Attack fails - adversary sees only encrypted data

---

#### 2. Man-in-the-Middle (MITM)

**Attack:** Adversary intercepts and modifies data in transit

**Defense:**
- ✅ WebRTC DTLS provides authenticated encryption
- ✅ SHA-256 integrity check detects tampering
- ✅ GCM authentication tag validates each chunk

**Result:** ❌ Attack fails - tampering detected

---

#### 3. Server-Side File Access

**Attack:** Malicious server operator tries to read files

**Defense:**
- ✅ Files never sent to server
- ✅ P2P transfer via WebRTC DataChannel
- ✅ Server only handles signaling messages

**Result:** ❌ Attack fails - server has no access

---

#### 4. Key Extraction from Signaling

**Attack:** Adversary monitors WebSocket signaling

**Defense:**
- ✅ Encryption keys transmitted via DataChannel (not WebSocket)
- ✅ DataChannel has separate DTLS encryption
- ✅ Keys never appear in signaling messages

**Result:** ❌ Attack fails - keys not visible in signaling

---

#### 5. Replay Attack

**Attack:** Adversary captures and replays old encrypted chunks

**Defense:**
- ✅ Sessions are ephemeral (expire after 30 minutes)
- ✅ Keys generated fresh per session
- ✅ DataChannel connection is stateful

**Result:** ❌ Attack fails - old data rejected

---

#### 6. File Tampering

**Attack:** Adversary modifies encrypted chunks

**Defense:**
- ✅ GCM authentication tags on each chunk
- ✅ SHA-256 hash on complete file
- ✅ Any modification detected

**Result:** ❌ Attack fails - integrity check fails

---

### Attacks Outside Scope

❗ **Client-Side Attacks** (not protected):
- Compromised browser or device
- Malicious browser extensions
- Keyloggers, screen capture
- Physical access to unlocked device

❗ **JavaScript Trust** (assumption):
- Assumes the served JavaScript is not malicious
- In production, use HTTPS + Subresource Integrity (SRI)

❗ **Denial of Service**:
- No protection against connection flooding
- Rate limiting needed in production

---

## Security Guarantees

### What We Guarantee ✅

1. **Confidentiality**
   - File content encrypted end-to-end
   - Server cannot read file data
   - Network eavesdropper cannot read file data

2. **Integrity**
   - File tampering detected via SHA-256
   - Chunk tampering detected via GCM tags
   - Receiver verifies file is unmodified

3. **Ephemeral Sessions**
   - Keys exist only in browser memory
   - No persistence after session ends
   - Forward secrecy via DTLS

4. **Zero-Knowledge Server**
   - Server never sees plaintext
   - Server never sees encryption keys
   - Server only routes signaling messages

### What We Do NOT Guarantee ❌

1. **Sender/Receiver Authentication**
   - No verification of peer identity
   - Session ID is the only authentication
   - Optional: Add password/PIN protection

2. **Anonymity**
   - IP addresses visible to server
   - WebRTC may leak local IPs
   - Use VPN/Tor for anonymity (may break WebRTC)

3. **Protection Against Endpoint Compromise**
   - Malicious browser = full compromise
   - Malicious JavaScript = full compromise
   - Physical access = potential compromise

---

## Known Limitations

### 1. JavaScript Trust Model

**Issue:** Browsers execute JavaScript from the server.

**Risk:** If server is compromised, it could serve malicious JS that exfiltrates data.

**Mitigations:**
- Deploy with HTTPS + valid certificate
- Use Content Security Policy (CSP)
- Use Subresource Integrity (SRI) for external scripts
- Open-source code for transparency

### 2. WebRTC IP Leakage

**Issue:** WebRTC may reveal local/public IP addresses.

**Risk:** Anonymity compromise (not confidentiality).

**Mitigations:**
- VPN usage (may break P2P connection)
- Browser settings to disable WebRTC IP leakage
- Use TURN server (hides direct IP)

### 3. Large File Performance

**Issue:** Encrypting/decrypting large files in browser consumes memory.

**Risk:** Browser may slow down or crash.

**Mitigations:**
- 200MB file size limit (configurable)
- Streaming encryption (already implemented)
- Web Workers for off-main-thread processing (future enhancement)

### 4. Session Hijacking

**Issue:** Session IDs are bearer tokens.

**Risk:** Anyone with session ID can join.

**Mitigations:**
- Session IDs are 128-bit random (hard to guess)
- Sessions expire after 30 minutes
- Optional: Add password/PIN protection (future enhancement)

---

## Cryptographic Implementation Details

### Web Crypto API Usage

All cryptographic operations use the browser's native Web Crypto API:

```javascript
// Key Generation
const key = await crypto.subtle.generateKey(
    {
        name: 'AES-GCM',
        length: 256
    },
    true,  // extractable
    ['encrypt', 'decrypt']
);

// Encryption
const encrypted = await crypto.subtle.encrypt(
    {
        name: 'AES-GCM',
        iv: chunkIV
    },
    key,
    plaintext
);

// Decryption
const decrypted = await crypto.subtle.decrypt(
    {
        name: 'AES-GCM',
        iv: chunkIV
    },
    key,
    ciphertext
);

// Hashing
const hash = await crypto.subtle.digest('SHA-256', data);
```

**Security Benefits:**
- ✅ Hardware-accelerated (AES-NI on x86)
- ✅ Constant-time operations (prevents timing attacks)
- ✅ Browser-native (no external crypto libraries)
- ✅ Well-audited implementations

---

## Security Best Practices (Deployment)

### For Production Use

1. **Use HTTPS**
   ```
   - WebRTC requires secure context
   - Prevents MITM on initial page load
   - Protects signaling channel
   ```

2. **Content Security Policy**
   ```html
   <meta http-equiv="Content-Security-Policy" 
         content="default-src 'self'; 
                  connect-src 'self' wss:; 
                  script-src 'self'">
   ```

3. **Subresource Integrity**
   ```html
   <script src="app.js" 
           integrity="sha384-..."
           crossorigin="anonymous"></script>
   ```

4. **Rate Limiting**
   ```go
   // Limit session creation
   // Limit WebSocket connections per IP
   // Timeout idle connections
   ```

5. **Monitoring**
   ```
   - Log suspicious activities
   - Monitor session creation rates
   - Alert on anomalies
   ```

---

## Conclusion

The Secure P2P File Vault provides strong end-to-end encryption for browser-based file transfers. While it has some inherent limitations (JavaScript trust, endpoint security), it effectively protects against network-level attacks and ensures the server never sees file content or encryption keys.

**Key Takeaway:** This system is secure against **passive and active network adversaries** but assumes **trusted endpoints and browser environments**.

---

## References

- [AES-GCM Specification (NIST SP 800-38D)](https://nvlpubs.nist.gov/nistpubs/Legacy/SP/nistspecialpublication800-38d.pdf)
- [Web Crypto API Specification](https://www.w3.org/TR/WebCryptoAPI/)
- [WebRTC Security Architecture](https://datatracker.ietf.org/doc/html/rfc8826)
- [SHA-256 Specification (FIPS 180-4)](https://csrc.nist.gov/publications/detail/fips/180/4/final)

---

**Last Updated:** January 2026  
**Version:** 1.0
