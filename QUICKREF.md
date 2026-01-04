# 📋 Quick Reference

Quick command reference for the Secure P2P File Vault project.

---

## 🚀 Getting Started

```bash
# Start the server
cd backend
go run main.go

# Or use the quick start script
./start.sh
```

**Access:** `http://localhost:8080`

---

## 📁 Project Structure

```
file-vault/
├── backend/
│   ├── main.go              # Signaling server
│   └── go.mod               # Dependencies
├── frontend/
│   ├── index.html           # UI
│   ├── app.js               # WebRTC + Crypto
│   └── styles.css           # Styling
├── README.md                # Main documentation
├── SECURITY.md              # Security details
├── ARCHITECTURE.md          # Architecture diagrams
├── TESTING.md               # Testing guide
└── start.sh                 # Quick start script
```

---

## 🔑 Key Concepts

### Signaling Flow
```
1. Sender creates session → gets SessionID
2. Receiver joins with SessionID
3. WebRTC SDP offer/answer exchange
4. ICE candidates exchanged
5. P2P connection established
6. File transfer begins
```

### Encryption
```
Algorithm: AES-GCM-256
Key Size: 256 bits
IV Size: 96 bits
Chunk Size: 64 KB
Hash: SHA-256
```

### Message Types
```javascript
// WebSocket (Signaling)
{ type: "join", payload: { sessionId, role } }
{ type: "offer", payload: <SDP> }
{ type: "answer", payload: <SDP> }
{ type: "ice", payload: <candidate> }
{ type: "ready", sessionId, peerId }
{ type: "error", payload: { message } }

// DataChannel (Transfer)
{ type: "metadata", data: { key, iv, hash, name, size } }
<Binary encrypted chunks>
{ type: "complete" }
```

---

## 🛠️ Development

### Backend Commands

```bash
# Install dependencies
go mod download

# Run server
go run main.go

# Build binary
go build -o file-vault

# Run binary
./file-vault

# Test (future)
go test ./...
```

### Frontend Development

```bash
# Serve frontend (server already does this)
# Just edit files and refresh browser

# No build step needed (vanilla JS)
```

---

## 🔍 Debugging

### Backend Logs
```bash
# Server logs show:
# - Session creation/cleanup
# - Peer connections/disconnections
# - Signaling messages (not file data!)

# Check logs:
go run main.go 2>&1 | tee server.log
```

### Browser Console
```javascript
// Check WebRTC states
console.log(app.pc.connectionState);
console.log(app.dataChannel.readyState);

// Check encryption
console.log(app.encryptionKey);

// Monitor transfer
console.log(app.currentChunk, '/', app.totalChunks);
```

---

## 🧪 Testing

### Quick Test (Single Machine)
```bash
# Terminal 1
cd backend && go run main.go

# Browser 1 (Sender)
http://localhost:8080
→ Click "Send File"
→ Copy Session ID

# Browser 2 (Receiver)
http://localhost:8080
→ Click "Receive File"
→ Paste Session ID
→ Click Connect

# Browser 1
→ Select/drop file

# Browser 2
→ Wait for transfer
→ Download file
```

### Network Test (Different Devices)
```bash
# Find server IP
ip addr show | grep "inet "
# Example: 192.168.1.100

# On other device
http://192.168.1.100:8080
```

---

## 🔐 Security Checklist

- [x] AES-GCM-256 encryption
- [x] SHA-256 integrity check
- [x] DTLS for DataChannel
- [x] Random session IDs (128-bit)
- [x] Ephemeral keys (not persisted)
- [x] Zero server-side file storage
- [x] Session expiration (30 min)

---

## 🐛 Troubleshooting

### Problem: Connection fails
```
Solution:
1. Check both peers joined same session
2. Verify firewall allows WebRTC
3. Try different STUN server
4. Check browser console for errors
```

### Problem: Transfer slow
```
Solution:
1. Check network speed
2. Try smaller file
3. Increase CHUNK_SIZE in app.js
4. Close other browser tabs
```

### Problem: Hash mismatch
```
Solution:
1. Retry transfer
2. Check network stability
3. Verify no packet loss
4. Try smaller file first
```

### Problem: Session not found
```
Solution:
1. Check session ID is correct
2. Verify session hasn't expired (30 min)
3. Create new session
4. Check server is running
```

---

## 📊 Configuration

### Backend (main.go)

```go
// Adjust these constants:
const (
    ServerPort = ":8080"
    SessionExpiry = 30 * time.Minute
    MaxSessionAge = 30 * time.Minute
)

// STUN server
iceServers: []
    { urls: 'stun:stun.l.google.com:19302' }
```

### Frontend (app.js)

```javascript
// Adjust these constants:
const CHUNK_SIZE = 64 * 1024;        // 64KB
const MAX_FILE_SIZE = 200 * 1024 * 1024; // 200MB

// WebRTC config
const config = {
    iceServers: [
        { urls: 'stun:stun.l.google.com:19302' }
    ]
};
```

---

## 🌐 Deployment

### Development
```bash
# Just run locally
go run main.go
```

### Production (Basic)
```bash
# Build binary
go build -o file-vault

# Run with systemd/supervisor
./file-vault
```

### Production (HTTPS)
```nginx
# nginx reverse proxy
server {
    listen 443 ssl http2;
    server_name vault.example.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

---

## 📚 Documentation Links

- [README.md](README.md) - Main documentation
- [SECURITY.md](SECURITY.md) - Security architecture
- [ARCHITECTURE.md](ARCHITECTURE.md) - System diagrams
- [TESTING.md](TESTING.md) - Testing guide

### External Resources

- [WebRTC Docs](https://webrtc.org/)
- [Web Crypto API](https://developer.mozilla.org/en-US/docs/Web/API/Web_Crypto_API)
- [Gorilla WebSocket](https://github.com/gorilla/websocket)
- [AES-GCM Spec](https://nvlpubs.nist.gov/nistpubs/Legacy/SP/nistspecialpublication800-38d.pdf)

---

## 🎯 Common Tasks

### Add new STUN server
```javascript
// In app.js, createPeerConnection():
const config = {
    iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        { urls: 'stun:stun1.l.google.com:19302' }, // Add this
    ]
};
```

### Increase max file size
```javascript
// In app.js:
const MAX_FILE_SIZE = 500 * 1024 * 1024; // 500MB
```

### Change server port
```go
// In main.go:
port := ":3000"  // Change from :8080
```

### Add TURN server (for strict NAT)
```javascript
const config = {
    iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        {
            urls: 'turn:turn.example.com:3478',
            username: 'user',
            credential: 'pass'
        }
    ]
};
```

---

## 📈 Performance Tips

1. **Optimize chunk size**: Adjust `CHUNK_SIZE` based on network
2. **Use Web Workers**: Offload encryption (future enhancement)
3. **Enable compression**: Consider LZ4/Brotli before encryption
4. **Load balancing**: Run multiple server instances
5. **CDN for frontend**: Serve static files from CDN

---

## 🔒 Security Best Practices

1. ✅ **Use HTTPS** in production (WebRTC requirement)
2. ✅ **Add rate limiting** to prevent abuse
3. ✅ **Implement CSP** (Content Security Policy)
4. ✅ **Add session passwords** (optional enhancement)
5. ✅ **Monitor server** for suspicious activity
6. ✅ **Keep dependencies updated**

---

## 🤝 Contributing

```bash
# Fork the repository
git clone https://github.com/yourusername/file-vault

# Create feature branch
git checkout -b feature/my-feature

# Make changes and commit
git commit -am "Add my feature"

# Push and create PR
git push origin feature/my-feature
```

---

## 📞 Support

**Issues?**
- Check [TESTING.md](TESTING.md) first
- Review browser console logs
- Check server logs
- Open GitHub issue with details

**Questions?**
- Read [README.md](README.md)
- Check [ARCHITECTURE.md](ARCHITECTURE.md)
- Review [SECURITY.md](SECURITY.md)

---

## 🎓 Learning Resources

### Understand WebRTC
1. Read [WebRTC Architecture](ARCHITECTURE.md)
2. Experiment with different browsers
3. Monitor network traffic
4. Debug connection issues

### Understand Cryptography
1. Read [Security Model](SECURITY.md)
2. Study AES-GCM implementation
3. Trace encryption flow in code
4. Test integrity verification

### Understand Go
1. Study [main.go](backend/main.go)
2. Learn Gorilla WebSocket
3. Understand concurrency (goroutines)
4. Practice session management

---

**Quick Reference Version 1.0**  
**Last Updated: January 2026**
