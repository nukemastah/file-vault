/**
 * Secure P2P File Vault - Main Application
 * 
 * Implements:
 * - WebRTC peer connection with DataChannel
 * - End-to-end encryption using AES-GCM
 * - File chunking and reassembly
 * - SHA-256 integrity verification
 */

const CHUNK_SIZE = 64 * 1024; // 64KB chunks
const MAX_FILE_SIZE = 200 * 1024 * 1024; // 200MB limit

class SecureFileVault {
    constructor() {
        this.role = null;
        this.sessionId = null;
        this.ws = null;
        this.pc = null;
        this.dataChannel = null;
        this.file = null;
        this.encryptionKey = null;
        this.iv = null;
        
        // Transfer state
        this.receivedChunks = [];
        this.totalChunks = 0;
        this.currentChunk = 0;
        this.fileMetadata = null;
        this.startTime = null;
        this.bytesTransferred = 0;
        this.iceCandidates = [];
        this.remoteDescriptionSet = false;
        
        this.init();
    }

    init() {
        // Setup drag and drop
        const dropzone = document.getElementById('dropzone');
        const fileInput = document.getElementById('fileInput');
        
        if (dropzone) {
            dropzone.addEventListener('click', () => fileInput.click());
            dropzone.addEventListener('dragover', (e) => {
                e.preventDefault();
                dropzone.classList.add('dragover');
            });
            dropzone.addEventListener('dragleave', () => {
                dropzone.classList.remove('dragover');
            });
            dropzone.addEventListener('drop', (e) => {
                e.preventDefault();
                dropzone.classList.remove('dragover');
                if (e.dataTransfer.files.length > 0) {
                    this.handleFileSelect(e.dataTransfer.files[0]);
                }
            });
        }
        
        if (fileInput) {
            fileInput.addEventListener('change', (e) => {
                if (e.target.files.length > 0) {
                    this.handleFileSelect(e.target.files[0]);
                }
            });
        }

        // Setup session input
        const sessionInput = document.getElementById('sessionInput');
        if (sessionInput) {
            sessionInput.addEventListener('keypress', (e) => {
                if (e.key === 'Enter') {
                    this.joinSession();
                }
            });
        }
    }

    // ========================================
    // Role Selection & Session Management
    // ========================================

    selectRole(role) {
        this.role = role;
        this.hideScreen('roleSelection');
        
        if (role === 'sender') {
            this.showScreen('senderScreen');
            this.createSession();
        } else {
            this.showScreen('receiverScreen');
        }
    }

    async createSession() {
        try {
            this.updateStatus('Creating session...');
            
            // Connect to signaling server
            this.connectWebSocket();
            
            // Generate encryption key for this session
            this.encryptionKey = await this.generateEncryptionKey();
            this.iv = crypto.getRandomValues(new Uint8Array(12));
            
        } catch (error) {
            this.showError('Failed to create session: ' + error.message);
        }
    }

    async joinSession() {
        const sessionInput = document.getElementById('sessionInput');
        const sessionId = sessionInput.value.trim();
        
        if (!sessionId) {
            this.showError('Please enter a session ID');
            return;
        }
        
        this.sessionId = sessionId;
        document.getElementById('sessionIdInput').classList.add('hidden');
        document.getElementById('receiverStatus').classList.remove('hidden');
        this.updateReceiverStatus('Connecting to session...');
        
        this.connectWebSocket();
    }

    // ========================================
    // WebSocket Signaling
    // ========================================

    connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        this.ws = new WebSocket(wsUrl);
        
        this.ws.onopen = () => {
            console.log('WebSocket connected');
            this.sendSignal({
                type: 'join',
                payload: {
                    sessionId: this.sessionId,
                    role: this.role
                }
            });
        };
        
        this.ws.onmessage = (event) => {
            const msg = JSON.parse(event.data);
            this.handleSignalMessage(msg);
        };
        
        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            this.showError('Connection error');
        };
        
        this.ws.onclose = () => {
            console.log('WebSocket closed');
        };
    }

    sendSignal(message) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(message));
        }
    }

    async handleSignalMessage(msg) {
        console.log('Signal received:', msg.type, msg);
        
        switch (msg.type) {
            case 'ready':
                this.sessionId = msg.sessionId;
                if (this.role === 'sender') {
                    document.getElementById('sessionIdDisplay').classList.remove('hidden');
                    document.getElementById('sessionIdText').textContent = this.sessionId;
                    this.updateStatus('Waiting for receiver...');
                    this.createPeerConnection();
                } else {
                    this.updateReceiverStatus('Connecting...');
                    this.createPeerConnection();
                }
                break;
                
            case 'peer-joined':
                // Receiver has joined - sender should create and send offer
                console.log('🎉 Receiver joined! Creating WebRTC offer...');
                this.updateStatus('Receiver connected! Establishing secure connection...');
                // Small delay to ensure peer connection is ready
                setTimeout(() => this.createOffer(), 100);
                break;
                
            case 'offer':
                await this.handleOffer(msg.payload);
                break;
                
            case 'answer':
                await this.handleAnswer(msg.payload);
                break;
                
            case 'ice':
                await this.handleICE(msg.payload);
                break;
                
            case 'error':
                const errorMsg = typeof msg.payload === 'string' 
                    ? JSON.parse(msg.payload).message 
                    : (msg.payload && msg.payload.message) || 'Unknown error';
                console.error('Error from server:', errorMsg);
                this.showError(errorMsg);
                break;
                
            case 'expired':
                this.showError('Session expired');
                break;
                
            default:
                console.warn('Unknown message type:', msg.type);
        }
    }

    // ========================================
    // WebRTC Connection
    // ========================================

    createPeerConnection() {
        const config = {
            iceServers: [
                { urls: 'stun:stun.l.google.com:19302' }
            ]
        };
        
        this.pc = new RTCPeerConnection(config);
        
        this.pc.onicecandidate = (event) => {
            if (event.candidate) {
                this.sendSignal({
                    type: 'ice',
                    sessionId: this.sessionId,
                    payload: event.candidate
                });
            }
        };
        
        this.pc.oniceconnectionstatechange = () => {
            console.log('ICE connection state:', this.pc.iceConnectionState);
            if (this.pc.iceConnectionState === 'connected' || this.pc.iceConnectionState === 'completed') {
                console.log('ICE connected!');
            }
        };
        
        this.pc.onconnectionstatechange = () => {
            console.log('Connection state:', this.pc.connectionState);
            if (this.pc.connectionState === 'connected') {
                console.log('Peer connection established!');
                if (this.role === 'sender') {
                    this.updateStatus('Connected! Select a file to send');
                    document.getElementById('fileUploadArea').classList.remove('hidden');
                } else {
                    this.updateReceiverStatus('Connected! Waiting for file...');
                }
            } else if (this.pc.connectionState === 'failed') {
                this.showError('Connection failed');
            } else if (this.pc.connectionState === 'disconnected') {
                this.showError('Connection disconnected');
            }
        };
        
        if (this.role === 'sender') {
            // Sender creates data channel
            // Offer will be created when receiver joins (peer-joined event)
            this.dataChannel = this.pc.createDataChannel('fileTransfer');
            this.setupDataChannel();
        } else {
            // Receiver waits for data channel
            this.pc.ondatachannel = (event) => {
                this.dataChannel = event.channel;
                this.setupDataChannel();
            };
        }
    }

    setupDataChannel() {
        console.log('🔗 Setting up DataChannel...');
        this.dataChannel.binaryType = 'arraybuffer';
        
        this.dataChannel.onopen = () => {
            console.log('✅ DataChannel opened - ready for transfer!');
            
            // Update UI when DataChannel is ready
            if (this.role === 'sender') {
                this.updateStatus('🔒 Connected! Select a file to send');
                document.getElementById('fileUploadArea').classList.remove('hidden');
            } else {
                this.updateReceiverStatus('🔒 Connected! Waiting for file...');
            }
        };
        
        this.dataChannel.onclose = () => {
            console.log('DataChannel closed');
        };
        
        this.dataChannel.onerror = (error) => {
            console.error('DataChannel error:', error);
            this.showError('Transfer error');
        };
        
        if (this.role === 'receiver') {
            this.dataChannel.onmessage = (event) => {
                this.handleDataChannelMessage(event.data);
            };
        }
    }

    async createOffer() {
        try {
            console.log('📡 Creating WebRTC offer...');
            const offer = await this.pc.createOffer();
            console.log('📡 Setting local description...');
            await this.pc.setLocalDescription(offer);
            console.log('📡 Sending offer to receiver...');
            this.sendSignal({
                type: 'offer',
                sessionId: this.sessionId,
                payload: offer
            });
            console.log('📡 Offer sent!');
        } catch (error) {
            console.error('Create offer error:', error);
            this.showError('Failed to create offer');
        }
    }

    async handleOffer(offer) {
        try {
            await this.pc.setRemoteDescription(new RTCSessionDescription(offer));
            
            // Process buffered ICE candidates
            for (const candidate of this.iceCandidates) {
                await this.pc.addIceCandidate(new RTCIceCandidate(candidate));
            }
            this.iceCandidates = [];
            
            const answer = await this.pc.createAnswer();
            await this.pc.setLocalDescription(answer);
            this.sendSignal({
                type: 'answer',
                sessionId: this.sessionId,
                payload: answer
            });
        } catch (error) {
            console.error('Handle offer error:', error);
        }
    }

    async handleAnswer(answer) {
        try {
            await this.pc.setRemoteDescription(new RTCSessionDescription(answer));
            
            // Process buffered ICE candidates
            for (const candidate of this.iceCandidates) {
                await this.pc.addIceCandidate(new RTCIceCandidate(candidate));
            }
            this.iceCandidates = [];
        } catch (error) {
            console.error('Handle answer error:', error);
        }
    }

    async handleICE(candidate) {
        try {
            if (this.pc.remoteDescription) {
                await this.pc.addIceCandidate(new RTCIceCandidate(candidate));
            } else {
                // Buffer candidates until remote description is set
                this.iceCandidates.push(candidate);
            }
        } catch (error) {
            console.error('Handle ICE error:', error);
        }
    }

    // ========================================
    // File Handling & Encryption
    // ========================================

    async handleFileSelect(file) {
        if (file.size > MAX_FILE_SIZE) {
            this.showError(`File too large. Maximum size is ${MAX_FILE_SIZE / 1024 / 1024}MB`);
            return;
        }
        
        this.file = file;
        
        // Display file info
        document.getElementById('fileUploadArea').classList.add('hidden');
        document.getElementById('fileInfo').classList.remove('hidden');
        document.getElementById('fileName').textContent = file.name;
        document.getElementById('fileSize').textContent = this.formatBytes(file.size);
        
        // Start transfer
        await this.sendFile();
    }

    async sendFile() {
        try {
            this.updateStatus('Encrypting and sending file...');
            document.getElementById('transferProgress').classList.remove('hidden');
            
            // Calculate hash of original file
            const arrayBuffer = await this.file.arrayBuffer();
            const hash = await this.calculateHash(arrayBuffer);
            
            // Prepare metadata
            const metadata = {
                name: this.file.name,
                size: this.file.size,
                type: this.file.type,
                hash: hash,
                encryptionKey: await this.exportKey(this.encryptionKey),
                iv: Array.from(this.iv)
            };
            
            // Send metadata first
            this.dataChannel.send(JSON.stringify({ type: 'metadata', data: metadata }));
            
            // Encrypt and send file in chunks
            this.totalChunks = Math.ceil(this.file.size / CHUNK_SIZE);
            this.currentChunk = 0;
            this.startTime = Date.now();
            this.bytesTransferred = 0;
            
            for (let i = 0; i < this.totalChunks; i++) {
                const start = i * CHUNK_SIZE;
                const end = Math.min(start + CHUNK_SIZE, this.file.size);
                const chunk = arrayBuffer.slice(start, end);
                
                // Encrypt chunk
                const encryptedChunk = await this.encryptChunk(chunk, i);
                
                // Send encrypted chunk
                this.dataChannel.send(encryptedChunk);
                
                this.currentChunk = i + 1;
                this.bytesTransferred += chunk.byteLength;
                this.updateProgress();
                
                // Small delay to avoid overwhelming the channel
                await this.delay(1);
            }
            
            // Send completion signal
            this.dataChannel.send(JSON.stringify({ type: 'complete' }));
            
            document.getElementById('transferProgress').classList.add('hidden');
            document.getElementById('transferComplete').classList.remove('hidden');
            this.updateStatus('Transfer complete!');
            
        } catch (error) {
            console.error('Send file error:', error);
            this.showError('Failed to send file: ' + error.message);
        }
    }

    async handleDataChannelMessage(data) {
        if (typeof data === 'string') {
            const message = JSON.parse(data);
            
            if (message.type === 'metadata') {
                this.fileMetadata = message.data;
                
                // Import encryption key
                this.encryptionKey = await this.importKey(message.data.encryptionKey);
                this.iv = new Uint8Array(message.data.iv);
                
                // Display file info
                document.getElementById('receiverFileInfo').classList.remove('hidden');
                document.getElementById('receiverFileName').textContent = message.data.name;
                document.getElementById('receiverFileSize').textContent = this.formatBytes(message.data.size);
                
                document.getElementById('receiverProgress').classList.remove('hidden');
                this.updateReceiverStatus('Receiving file...');
                
                this.totalChunks = Math.ceil(message.data.size / CHUNK_SIZE);
                this.receivedChunks = [];
                this.startTime = Date.now();
                this.bytesTransferred = 0;
                
            } else if (message.type === 'complete') {
                await this.reassembleFile();
            }
        } else {
            // Encrypted chunk received
            this.receivedChunks.push(data);
            this.bytesTransferred += data.byteLength;
            this.updateReceiverProgress();
        }
    }

    async reassembleFile() {
        try {
            this.updateReceiverStatus('Decrypting and verifying...');
            
            // Decrypt all chunks
            const decryptedChunks = [];
            for (let i = 0; i < this.receivedChunks.length; i++) {
                const decrypted = await this.decryptChunk(this.receivedChunks[i], i);
                decryptedChunks.push(decrypted);
            }
            
            // Combine chunks
            const totalSize = decryptedChunks.reduce((sum, chunk) => sum + chunk.byteLength, 0);
            const fileData = new Uint8Array(totalSize);
            let offset = 0;
            for (const chunk of decryptedChunks) {
                fileData.set(new Uint8Array(chunk), offset);
                offset += chunk.byteLength;
            }
            
            // Verify hash
            const hash = await this.calculateHash(fileData.buffer);
            if (hash !== this.fileMetadata.hash) {
                throw new Error('File integrity check failed!');
            }
            
            // Create blob and download link
            const blob = new Blob([fileData], { type: this.fileMetadata.type });
            const url = URL.createObjectURL(blob);
            
            document.getElementById('receiverProgress').classList.add('hidden');
            document.getElementById('receiverComplete').classList.remove('hidden');
            this.updateReceiverStatus('File received and verified!');
            
            document.getElementById('downloadBtn').onclick = () => {
                const a = document.createElement('a');
                a.href = url;
                a.download = this.fileMetadata.name;
                a.click();
            };
            
        } catch (error) {
            console.error('Reassemble error:', error);
            this.showError('Failed to decrypt file: ' + error.message);
        }
    }

    // ========================================
    // Cryptography Functions
    // ========================================

    async generateEncryptionKey() {
        return await crypto.subtle.generateKey(
            {
                name: 'AES-GCM',
                length: 256
            },
            true,
            ['encrypt', 'decrypt']
        );
    }

    async exportKey(key) {
        const exported = await crypto.subtle.exportKey('raw', key);
        return Array.from(new Uint8Array(exported));
    }

    async importKey(keyData) {
        return await crypto.subtle.importKey(
            'raw',
            new Uint8Array(keyData),
            { name: 'AES-GCM' },
            true,
            ['encrypt', 'decrypt']
        );
    }

    async encryptChunk(chunk, chunkIndex) {
        // Create unique IV for this chunk
        const chunkIv = new Uint8Array(12);
        chunkIv.set(this.iv);
        // Mix in chunk index for uniqueness
        const indexBytes = new Uint8Array(new Uint32Array([chunkIndex]).buffer);
        for (let i = 0; i < indexBytes.length; i++) {
            chunkIv[i] ^= indexBytes[i];
        }
        
        const encrypted = await crypto.subtle.encrypt(
            {
                name: 'AES-GCM',
                iv: chunkIv
            },
            this.encryptionKey,
            chunk
        );
        
        return encrypted;
    }

    async decryptChunk(encryptedChunk, chunkIndex) {
        // Recreate the same IV used for encryption
        const chunkIv = new Uint8Array(12);
        chunkIv.set(this.iv);
        const indexBytes = new Uint8Array(new Uint32Array([chunkIndex]).buffer);
        for (let i = 0; i < indexBytes.length; i++) {
            chunkIv[i] ^= indexBytes[i];
        }
        
        const decrypted = await crypto.subtle.decrypt(
            {
                name: 'AES-GCM',
                iv: chunkIv
            },
            this.encryptionKey,
            encryptedChunk
        );
        
        return decrypted;
    }

    async calculateHash(arrayBuffer) {
        const hashBuffer = await crypto.subtle.digest('SHA-256', arrayBuffer);
        const hashArray = Array.from(new Uint8Array(hashBuffer));
        return hashArray.map(b => b.toString(16).padStart(2, '0')).join('');
    }

    // ========================================
    // UI Helpers
    // ========================================

    showScreen(screenId) {
        document.querySelectorAll('.screen').forEach(s => s.classList.remove('active'));
        document.getElementById(screenId).classList.add('active');
    }

    hideScreen(screenId) {
        document.getElementById(screenId).classList.remove('active');
    }

    updateStatus(message) {
        console.log('Sender status update:', message);
        const statusText = document.getElementById('statusText');
        const statusDot = document.getElementById('statusDot');
        if (statusText) {
            statusText.textContent = message;
            console.log('Status text updated to:', statusText.textContent);
        }
        if (statusDot) {
            statusDot.className = 'status-dot';
            if (message.includes('Connected')) {
                statusDot.classList.add('connected');
            } else if (message.includes('Waiting')) {
                statusDot.classList.add('waiting');
            }
        }
    }

    updateReceiverStatus(message) {
        const statusText = document.getElementById('receiverStatusText');
        const statusDot = document.getElementById('receiverStatusDot');
        if (statusText) statusText.textContent = message;
        if (statusDot) {
            statusDot.className = 'status-dot';
            if (message.includes('Connected')) {
                statusDot.classList.add('connected');
            } else if (message.includes('Waiting') || message.includes('Receiving')) {
                statusDot.classList.add('waiting');
            }
        }
    }

    updateProgress() {
        const percent = Math.round((this.currentChunk / this.totalChunks) * 100);
        document.getElementById('progressPercent').textContent = `${percent}%`;
        document.getElementById('progressFill').style.width = `${percent}%`;
        
        const elapsed = (Date.now() - this.startTime) / 1000;
        const speed = this.bytesTransferred / elapsed;
        const remaining = (this.file.size - this.bytesTransferred) / speed;
        
        document.getElementById('progressText').textContent = 'Sending...';
        document.getElementById('transferSpeed').textContent = `${this.formatBytes(speed)}/s`;
        document.getElementById('transferEta').textContent = `ETA: ${this.formatTime(remaining)}`;
    }

    updateReceiverProgress() {
        const percent = Math.round((this.receivedChunks.length / this.totalChunks) * 100);
        document.getElementById('receiverProgressPercent').textContent = `${percent}%`;
        document.getElementById('receiverProgressFill').style.width = `${percent}%`;
        
        const elapsed = (Date.now() - this.startTime) / 1000;
        const speed = this.bytesTransferred / elapsed;
        const remaining = (this.fileMetadata.size - this.bytesTransferred) / speed;
        
        document.getElementById('receiverProgressText').textContent = 'Receiving...';
        document.getElementById('receiverTransferSpeed').textContent = `${this.formatBytes(speed)}/s`;
        document.getElementById('receiverTransferEta').textContent = `ETA: ${this.formatTime(remaining)}`;
    }

    formatBytes(bytes) {
        if (bytes === 0) return '0 Bytes';
        const k = 1024;
        const sizes = ['Bytes', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
    }

    formatTime(seconds) {
        if (!isFinite(seconds)) return '--';
        if (seconds < 60) return `${Math.round(seconds)}s`;
        const mins = Math.floor(seconds / 60);
        const secs = Math.round(seconds % 60);
        return `${mins}m ${secs}s`;
    }

    copySessionId(event) {
        const sessionId = document.getElementById('sessionIdText').textContent;
        navigator.clipboard.writeText(sessionId).then(() => {
            const btn = event.target;
            const originalText = btn.textContent;
            btn.textContent = '✓ Copied!';
            setTimeout(() => {
                btn.textContent = originalText;
            }, 2000);
        }).catch(err => {
            console.error('Failed to copy:', err);
            this.showError('Failed to copy Session ID');
        });
    }

    showError(message) {
        const errorBox = document.getElementById('errorBox');
        const errorText = document.getElementById('errorText');
        errorText.textContent = message;
        errorBox.classList.remove('hidden');
        
        setTimeout(() => {
            this.hideError();
        }, 5000);
    }

    hideError() {
        document.getElementById('errorBox').classList.add('hidden');
    }

    reset() {
        window.location.reload();
    }

    delay(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }
}

// Initialize app
const app = new SecureFileVault();
