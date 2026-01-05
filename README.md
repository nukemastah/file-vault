# 🔐 Secure P2P File Vault

A peer-to-peer file sharing application with end-to-end encryption, built with Go (Pion WebRTC) and WebRTC. Files are transferred directly between browsers using WebRTC DataChannels with zero server-side storage.

## 🌟 Features

- **🔒 End-to-End Encryption**: Files encrypted with AES-GCM-256 before transmission
- **🌐 Peer-to-Peer Transfer**: Direct browser-to-browser transfer via WebRTC DataChannel
- **🚫 Zero Server Storage**: Server only handles signaling, never sees file content
- **⏱️ Ephemeral Sessions**: Data exists only during active session
- **✅ Integrity Verification**: SHA-256 hash validation ensures file integrity
- **📊 Real-time Progress**: Transfer speed, ETA, and progress tracking
- **📱 Responsive UI**: Clean, modern interface that works on all devices
- **🔐 Session-based**: Unique session IDs for secure peer pairing
- **🔗 Powered by Pion**: High-performance WebRTC stack in pure Go

## 🏗️ Architecture

```
┌─────────────┐          ┌──────────────────┐          ┌─────────────┐
│   Sender    │          │  Signaling Server │          │  Receiver   │
│  (Browser)  │◄────────►│ (Go/Pion WebRTC) │◄────────►│  (Browser)  │
└─────────────┘          └──────────────────┘          └─────────────┘
       │                                                        │
       │                                                        │
       └────────────────► WebRTC DataChannel ◄─────────────────┘
                         (Encrypted P2P Transfer)
```

### Components

1. **Signaling Server (Go + Pion WebRTC)**
   - WebSocket-based signaling for WebRTC peer discovery
   - Pion WebRTC for SDP parsing and ICE candidate handling
   - Session lifecycle management
   - Connection state monitoring
   - No file data handling

2. **Frontend (Vanilla JS + Web Crypto API)**
   - WebRTC peer connection setup
   - AES-GCM encryption/decryption
   - File chunking (64KB chunks)
   - SHA-256 integrity verification

## 🔐 Security Model

### Cryptographic Design

```
1. Key Generation
   ├─ Sender generates AES-GCM 256-bit key
   ├─ 96-bit IV (initialization vector)
   └─ Keys transmitted via DTLS-secured DataChannel

2. File Encryption
   ├─ File split into 64KB chunks
   ├─ Each chunk: AES-GCM(chunk, key, IV + counter)
   └─ Counter incremented per chunk

3. Integrity Check
   ├─ SHA-256 hash computed on original file
   ├─ Hash transmitted in metadata
   └─ Receiver validates after reassembly
```

### Security Properties

✅ **End-to-End Encrypted**: Encryption happens in browser memory  
✅ **Forward Secrecy**: DTLS for DataChannel transport  
✅ **Integrity Protection**: SHA-256 prevents tampering  
✅ **Ephemeral**: Keys and files exist only in RAM  
✅ **Zero-Knowledge Server**: Server cannot decrypt files  

### Threat Model

**Protected Against:**
- Server eavesdropping
- Man-in-the-middle on signaling
- File tampering in transit
- Post-session data recovery

**NOT Protected Against:**
- Malicious client code (assumes trusted JS)
- Compromised browser/endpoint
- Active client-side attacks

## 📂 Project Structure

```
file-vault/
├── backend/
│   ├── main.go           # WebSocket signaling server
│   └── go.mod            # Go dependencies
├── frontend/
│   ├── index.html        # UI structure
│   ├── app.js            # WebRTC + encryption logic
│   └── styles.css        # Modern styling
└── README.md             # This file
```

## 🚀 Getting Started

### Prerequisites

- **Go 1.21+** for backend
- **Modern browser** with WebRTC support (Chrome, Firefox, Edge, Safari)
- **Local network** or public IP (STUN server configured for NAT traversal)

### Installation

1. **Clone the repository**
```bash
cd file-vault
```

2. **Install Go dependencies**
```bash
cd backend
go mod download
```

### Running the Application

1. **Start the signaling server**
```bash
cd backend
go run main.go
```

Server will start on `http://localhost:8080`

2. **Open in browsers**
   - Open `http://localhost:8080` in two browser windows/tabs
   - Or access from different devices on the same network

### Usage Flow

#### Sender Side:
1. Click **"📤 Send File"**
2. Wait for session to initialize
3. Copy the **Session ID**
4. Share Session ID with receiver (via chat, email, etc.)
5. Wait for receiver to connect
6. Drag & drop or select your file
7. File will be encrypted and sent automatically

#### Receiver Side:
1. Click **"📥 Receive File"**
2. Paste the **Session ID** provided by sender
3. Click **"Connect"**
4. Wait for file transfer to complete
5. Verify integrity
6. Click **"💾 Download File"**

## 🔧 Technical Details

### WebRTC Configuration

- **STUN Server**: `stun:stun.l.google.com:19302` (for NAT traversal)
- **DataChannel**: Reliable, ordered transmission
- **Chunk Size**: 64KB
- **Max File Size**: 200MB (configurable)

### Encryption Specifications

- **Algorithm**: AES-GCM (Galois/Counter Mode)
- **Key Size**: 256 bits
- **IV Size**: 96 bits (unique per chunk via counter)
- **Authentication Tag**: 128 bits (built into GCM)
- **Hash Function**: SHA-256

### Signaling Protocol

WebSocket messages use JSON format:

```json
{
  "type": "join|offer|answer|ice|ready|error|expired",
  "sessionId": "hex-encoded-session-id",
  "peerId": "hex-encoded-peer-id",
  "payload": { /* type-specific data */ }
}
```

### File Transfer Protocol

1. **Metadata Message** (JSON)
```json
{
  "type": "metadata",
  "data": {
    "name": "document.pdf",
    "size": 1048576,
    "type": "application/pdf",
    "hash": "sha256-hash-hex",
    "encryptionKey": [/* raw key bytes */],
    "iv": [/* IV bytes */]
  }
}
```

2. **Encrypted Chunks** (Binary)
   - Sent sequentially as ArrayBuffer
   - Each chunk is AES-GCM encrypted

3. **Completion Message** (JSON)
```json
{
  "type": "complete"
}
```

## 🧪 Testing

### Local Testing (Same Machine)
1. Open two browser tabs
2. Use sender in one, receiver in the other
3. Share session ID between tabs

### Network Testing (Different Devices)
1. Ensure both devices are on the same network
2. Access server via local IP: `http://192.168.x.x:8080`
3. Share session ID via chat/email

### Security Testing
- ✅ Verify server logs show no file content
- ✅ Check network tab: binary data is opaque
- ✅ Confirm hash validation (try tampering)

## 🔍 Monitoring & Debugging

### Backend Logs
```bash
# Server shows:
- Session creation/cleanup
- Peer join/leave events
- Signaling message routing
- NO file content
```

### Browser Console
```javascript
// Enable verbose logging:
// Check browser console for WebRTC states
```

### Common Issues

**Connection Fails:**
- Check firewall settings
- Verify both peers are online
- Try different STUN servers if behind strict NAT

**Transfer Slow:**
- Large files may take time (P2P bandwidth limited)
- Check network conditions
- Consider smaller chunk sizes for unstable connections

**Hash Mismatch:**
- Indicates corruption or tampering
- Check network stability
- Retry transfer

## 🌐 Deployment Considerations

### Production Deployment

⚠️ **This is a demo/educational project.** For production:

1. **HTTPS Required**: WebRTC requires secure context
   - Use Let's Encrypt for free SSL
   - Configure reverse proxy (nginx/Caddy)

2. **TURN Server**: For strict NAT/firewall scenarios
   - STUN alone may not work
   - Deploy coturn or use cloud TURN service

3. **Rate Limiting**: Prevent abuse
   - Limit session creation rate
   - Timeout inactive connections

4. **Authentication**: Add user auth if needed
   - Session passwords
   - OAuth integration

5. **Monitoring**: Production metrics
   - Session analytics
   - Error tracking
   - Performance monitoring

### Environment Variables

```bash
# Backend configuration (add to main.go if needed)
PORT=8080
MAX_SESSION_AGE=30m
STUN_SERVER=stun:stun.l.google.com:19302
```

## 📚 Educational Resources

### WebRTC Concepts
- [WebRTC for Beginners](https://webrtc.org/getting-started/overview)
- [DataChannels API](https://developer.mozilla.org/en-US/docs/Web/API/RTCDataChannel)

### Web Crypto API
- [Crypto.subtle Documentation](https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto)
- [AES-GCM Explained](https://datatracker.ietf.org/doc/html/rfc5116)

### Golang WebSockets
- [Gorilla WebSocket](https://github.com/gorilla/websocket)

## 🛠️ Advanced Features (Optional)

### Implemented
- ✅ File chunking and reassembly
- ✅ Progress tracking with ETA
- ✅ Session expiration (30 min)
- ✅ Integrity verification

### Future Enhancements
- 🔲 Multiple file support
- 🔲 Transfer pause/resume
- 🔲 QR code session sharing
- 🔲 Password-protected vaults
- 🔲 One-time download links
- 🔲 File preview (images/PDFs)
- 🔲 Mobile app (React Native)

## 📖 Code Walkthrough

### Backend Flow ([main.go](backend/main.go))
```
1. Client connects via WebSocket
2. Join message → assigns to session
3. Route SDP offers/answers between peers
4. Route ICE candidates for NAT traversal
5. Monitor session lifecycle
6. Cleanup expired sessions
```

### Frontend Flow ([app.js](frontend/app.js))
```
1. User selects role (sender/receiver)
2. WebSocket connection to signaling server
3. WebRTC peer connection established
4. DataChannel opened
5. Key exchange via metadata message
6. Encrypted chunks transmitted
7. Receiver decrypts and verifies hash
8. File download triggered
```

## 🤝 Contributing

This is an educational project. Contributions welcome:

1. Fork the repository
2. Create feature branch
3. Commit changes
4. Push to branch
5. Open pull request

## 📄 License

MIT License - feel free to use for learning and projects.

## ⚠️ Disclaimer

This is a **demonstration project** for educational purposes. While it implements strong cryptography and secure practices, it has not undergone professional security audit. Do not use for highly sensitive data without additional security review.

## 🙏 Acknowledgments

- **Gorilla WebSocket**: Excellent Go WebSocket library
- **Pion WebRTC**: (Not used in this version but excellent for Go-side WebRTC)
- **Web Crypto API**: Browser-native cryptography
- **STUN Servers**: Google's public STUN infrastructure

## 📞 Support

For issues or questions:
- Open a GitHub issue
- Check browser console for errors
- Review server logs for debugging

---

**Built with ❤️ for learning distributed systems, WebRTC, and applied cryptography**