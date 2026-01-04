# 🧪 Testing Guide

This guide helps you test the Secure P2P File Vault to verify all functionality and security properties.

---

## Quick Start Testing

### 1. Single Machine Test (Easiest)

**Setup:**
```bash
cd backend
go run main.go
```

**Test:**
1. Open `http://localhost:8080` in two different browser tabs/windows
2. **Tab 1 (Sender):**
   - Click "📤 Send File"
   - Copy the Session ID
3. **Tab 2 (Receiver):**
   - Click "📥 Receive File"
   - Paste the Session ID
   - Click "Connect"
4. **Tab 1:** Drag & drop a file (try 1-50MB)
5. **Tab 2:** Wait for transfer, then download

**Expected Result:** ✅ File transfers successfully, hash verified

---

## Functional Tests

### Test 1: Small File Transfer (< 1MB)

**Purpose:** Verify basic functionality

**Steps:**
1. Send a small text file (e.g., 100KB)
2. Monitor browser console for logs
3. Verify progress bar updates
4. Download and compare files

**Validation:**
```bash
# Compare original and downloaded file
sha256sum original.txt
sha256sum downloaded.txt
# Should match!
```

---

### Test 2: Medium File Transfer (10-50MB)

**Purpose:** Verify chunking and progress tracking

**Steps:**
1. Send a medium file (e.g., 20MB video)
2. Observe progress percentage
3. Note transfer speed and ETA

**Expected Behavior:**
- Progress bar updates smoothly
- Transfer speed shown (KB/s or MB/s)
- ETA calculates correctly
- No browser freezing

---

### Test 3: Large File Transfer (100-200MB)

**Purpose:** Stress test chunk handling

**Steps:**
1. Send largest supported file (up to 200MB)
2. Monitor memory usage (browser dev tools)
3. Complete transfer

**Expected Behavior:**
- Transfer completes without errors
- Memory usage stays reasonable
- No browser crashes

---

### Test 4: Multiple File Types

**Purpose:** Verify format compatibility

**Test Files:**
- Text: `.txt`, `.md`, `.json`
- Images: `.jpg`, `.png`, `.gif`
- Documents: `.pdf`, `.docx`
- Archives: `.zip`, `.tar.gz`
- Executables: `.exe`, `.sh`
- Videos: `.mp4`, `.mkv`

**Validation:**
- All file types transfer successfully
- Downloaded files are identical to originals
- No corruption detected

---

### Test 5: Session Expiry

**Purpose:** Verify ephemeral nature

**Steps:**
1. Create a session
2. Wait 30+ minutes without connecting receiver
3. Try to join with Session ID

**Expected Behavior:**
- ❌ Session expired error
- Server logs show session cleanup

---

### Test 6: Connection Interruption

**Purpose:** Test error handling

**Test Cases:**
1. **Close sender tab mid-transfer**
   - Receiver should show error
   
2. **Close receiver tab before transfer**
   - Sender should detect disconnect
   
3. **Network disconnection**
   - Simulate by disabling WiFi
   - Should show connection failed

---

## Security Tests

### Test 7: Server Cannot Read Files

**Purpose:** Verify zero-knowledge property

**Steps:**
1. Enable verbose logging on server:
   ```go
   log.SetFlags(log.LstdFlags | log.Lshortfile)
   ```

2. Transfer a file with known content (e.g., "SECRET_DATA_12345")

3. Check server logs:
   ```bash
   grep -i "SECRET_DATA" server.log
   ```

**Expected Result:** ❌ No match found - server never sees content

---

### Test 8: Network Inspection

**Purpose:** Verify encryption in transit

**Steps:**
1. Open Browser DevTools → Network tab
2. Transfer a file
3. Inspect WebSocket messages
4. Check binary data in DataChannel

**Expected Observations:**
- Signaling messages are JSON (offer/answer/ICE)
- No plaintext file data in WebSocket
- DataChannel shows binary data (opaque)

---

### Test 9: Hash Verification

**Purpose:** Verify integrity protection

**Test A - Normal Transfer:**
1. Transfer file
2. Calculate hash locally:
   ```bash
   sha256sum original.txt
   ```
3. Check browser console for received hash
4. Should match ✅

**Test B - Tampered Data (Manual):**

Modify `app.js` to corrupt a chunk:
```javascript
// In sendFile(), after encryption:
if (i === 5) {
    // Corrupt chunk 5
    const corrupted = new Uint8Array(encryptedChunk);
    corrupted[0] ^= 0xFF;
    this.dataChannel.send(corrupted.buffer);
} else {
    this.dataChannel.send(encryptedChunk);
}
```

**Expected Result:** ❌ Hash mismatch error on receiver

---

### Test 10: Encryption Key Security

**Purpose:** Verify keys are not exposed

**Steps:**
1. Open Browser DevTools → Console
2. Transfer a file
3. Try to access keys:
   ```javascript
   // Type in console:
   app.encryptionKey
   ```

**Expected Behavior:**
- Key object exists but raw bytes not extractable via console
- Properly protected by Web Crypto API

---

### Test 11: Session ID Guessing

**Purpose:** Verify session security

**Attack Simulation:**
1. Create a session (gets Session ID)
2. Try to guess other session IDs:
   - Sequential: `sessionId + 1`
   - Random guesses

**Expected Result:** ❌ Random Session IDs (128-bit) make guessing infeasible

---

## Network Tests

### Test 12: Same Network (LAN)

**Setup:**
1. Find server machine's local IP:
   ```bash
   ip addr show | grep "inet "
   # Example: 192.168.1.100
   ```

2. On another device (same WiFi):
   - Open `http://192.168.1.100:8080`

**Test:**
- Complete file transfer between devices

**Expected Result:** ✅ Works via STUN for NAT traversal

---

### Test 13: Cross-Browser Compatibility

**Test Matrix:**

| Sender   | Receiver | Expected |
|----------|----------|----------|
| Chrome   | Chrome   | ✅ Works  |
| Chrome   | Firefox  | ✅ Works  |
| Chrome   | Edge     | ✅ Works  |
| Firefox  | Chrome   | ✅ Works  |
| Safari   | Chrome   | ✅ Works  |

**Steps:**
1. Test each combination
2. Verify full functionality

---

### Test 14: Mobile Devices

**Platforms:**
- Android Chrome
- iOS Safari
- Mobile Firefox

**Test:**
- Mobile → Desktop transfer
- Desktop → Mobile transfer
- Mobile → Mobile transfer

**Known Issues:**
- iOS may have WebRTC restrictions
- Background tabs may suspend connections

---

## Performance Tests

### Test 15: Transfer Speed

**Benchmark Setup:**
```javascript
// Add to app.js for detailed logging:
console.log(`Transfer complete: ${fileSize}B in ${duration}ms`);
console.log(`Average speed: ${fileSize / duration * 1000 / 1024 / 1024} MB/s`);
```

**Test:**
1. Transfer 50MB file
2. Record time taken
3. Calculate throughput

**Expected Performance:**
- LAN: 5-50 MB/s (depends on hardware)
- Same machine: 10-100 MB/s

---

### Test 16: Concurrent Sessions

**Purpose:** Test server scalability

**Setup:**
```bash
# Open multiple browser windows
# Create 5 simultaneous transfers
```

**Monitor:**
- Server CPU usage
- Memory usage
- All transfers complete successfully

**Expected Result:** ✅ Server handles multiple sessions

---

## Browser Console Tests

### Test 17: WebRTC Connection States

**Monitor in Console:**
```javascript
// Connection state transitions
pc.connectionState
// Should go: new → connecting → connected

// ICE gathering state
pc.iceGatheringState
// Should go: new → gathering → complete

// DataChannel state
dataChannel.readyState
// Should go: connecting → open
```

---

### Test 18: Error Scenarios

**Test Cases:**

1. **Invalid Session ID:**
   ```
   Input: "invalid123"
   Expected: "Session not found" error
   ```

2. **Oversized File:**
   ```
   Select: 300MB file
   Expected: "File too large" error
   ```

3. **Duplicate Role:**
   ```
   Two senders join same session
   Expected: "Sender already connected" error
   ```

4. **Network Timeout:**
   ```
   Disconnect network before connection
   Expected: Connection timeout error
   ```

---

## Automated Testing Script

Create `test.sh`:

```bash
#!/bin/bash

echo "🧪 Automated Testing Suite"
echo "=========================="

# Start server in background
cd backend
go run main.go &
SERVER_PID=$!
sleep 2

echo "✅ Server started (PID: $SERVER_PID)"

# Test 1: Server responds
echo "Testing server health..."
curl -f http://localhost:8080/ > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "✅ Server health check passed"
else
    echo "❌ Server health check failed"
fi

# Test 2: Session creation API
echo "Testing session creation..."
RESPONSE=$(curl -s -X POST http://localhost:8080/api/session)
SESSION_ID=$(echo $RESPONSE | grep -o '"sessionId":"[^"]*"' | cut -d'"' -f4)
if [ -n "$SESSION_ID" ]; then
    echo "✅ Session created: $SESSION_ID"
else
    echo "❌ Session creation failed"
fi

# Test 3: WebSocket connection
echo "Testing WebSocket endpoint..."
# (Requires websocat or similar tool)

# Cleanup
echo "Stopping server..."
kill $SERVER_PID
echo "✅ Tests complete"
```

Run with:
```bash
chmod +x test.sh
./test.sh
```

---

## Security Audit Checklist

- [ ] Server never logs file content
- [ ] Server never logs encryption keys
- [ ] WebSocket traffic contains only signaling
- [ ] DataChannel traffic is encrypted (DTLS)
- [ ] Hash verification catches corruption
- [ ] Session IDs are random (128-bit)
- [ ] Sessions expire after 30 minutes
- [ ] No file persistence on server
- [ ] No file persistence in browser after download
- [ ] Keys generated securely (crypto.subtle.generateKey)
- [ ] IVs are unique per chunk
- [ ] AES-GCM provides authenticated encryption

---

## Common Issues & Solutions

### Issue 1: Connection Fails

**Symptoms:** Peers don't connect, stuck on "Connecting..."

**Debug:**
1. Check browser console for errors
2. Verify both peers joined same session
3. Check firewall settings
4. Try different STUN server:
   ```javascript
   iceServers: [
       { urls: 'stun:stun1.l.google.com:19302' },
       { urls: 'stun:stun2.l.google.com:19302' }
   ]
   ```

---

### Issue 2: Transfer Slow

**Symptoms:** Transfer takes much longer than expected

**Causes:**
- Network congestion
- CPU bottleneck (encryption)
- Chunk size too small

**Solutions:**
- Test on better network
- Increase `CHUNK_SIZE` to 128KB
- Use smaller files for testing

---

### Issue 3: Hash Mismatch

**Symptoms:** "File integrity check failed" error

**Causes:**
- Network corruption
- Chunk order issue
- Memory corruption

**Debug:**
1. Enable verbose logging
2. Verify chunk indices
3. Try smaller file
4. Check browser console errors

---

### Issue 4: Browser Freezes

**Symptoms:** Browser becomes unresponsive during transfer

**Causes:**
- File too large
- Main thread blocking

**Solutions:**
- Reduce file size limit
- Consider Web Workers (future enhancement)
- Close other tabs

---

## Test Results Template

Document your test results:

```markdown
## Test Session: [Date]

**Environment:**
- Server: Go 1.21 on Linux
- Browser: Chrome 120
- Network: LAN (1 Gbps)

**Test Results:**

| Test | File Size | Duration | Speed | Status |
|------|-----------|----------|-------|--------|
| 1    | 1 MB      | 0.2s     | 5 MB/s| ✅      |
| 2    | 10 MB     | 1.5s     | 6.7 MB/s | ✅   |
| 3    | 50 MB     | 8s       | 6.25 MB/s | ✅  |
| 4    | 200 MB    | 35s      | 5.7 MB/s | ✅   |

**Security Tests:**
- [✅] Server logs clean (no file data)
- [✅] Hash verification passed
- [✅] Network inspection passed

**Issues Found:** None

**Notes:** All tests passed successfully.
```

---

## Continuous Testing

### Integration with CI/CD

Create `.github/workflows/test.yml`:

```yaml
name: Test Suite

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Setup Go
        uses: actions/setup-go@v2
        with:
          go-version: 1.21
      
      - name: Install dependencies
        run: cd backend && go mod download
      
      - name: Run server tests
        run: cd backend && go test ./...
      
      - name: Build
        run: cd backend && go build -v
```

---

## Conclusion

This testing guide covers:
- ✅ Functional testing (file transfers)
- ✅ Security testing (encryption, integrity)
- ✅ Performance testing (speed, scale)
- ✅ Error handling (edge cases)

**Next Steps:**
1. Run all tests systematically
2. Document results
3. Fix any issues found
4. Add automated tests as project matures

---

**Happy Testing! 🧪**
