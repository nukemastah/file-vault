package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

// Message types for WebSocket signaling
const (
	MsgTypeJoin       = "join"
	MsgTypeOffer      = "offer"
	MsgTypeAnswer     = "answer"
	MsgTypeICE        = "ice"
	MsgTypeReady      = "ready"
	MsgTypePeerJoined = "peer-joined"
	MsgTypeError      = "error"
	MsgTypeExpired    = "expired"
)

// SignalMessage represents WebSocket messages
type SignalMessage struct {
	Type      string          `json:"type"`
	SessionID string          `json:"sessionId,omitempty"`
	PeerID    string          `json:"peerId,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// Peer represents a connected client
type Peer struct {
	ID             string
	Conn           *websocket.Conn
	Role           string // "sender" or "receiver"
	SendChan       chan SignalMessage
	PeerConnection *webrtc.PeerConnection
	DataChannel    *webrtc.DataChannel
	SessionID      string
	mu             sync.Mutex
}

// Session represents a vault session
type Session struct {
	ID        string
	Sender    *Peer
	Receiver  *Peer
	CreatedAt time.Time
	mu        sync.RWMutex
}

// SessionManager manages all active vault sessions
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for demo (restrict in production)
		},
	}
	sessionManager *SessionManager

	// Pion WebRTC configuration
	webrtcConfig = webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{
					"stun:stun.l.google.com:19302",
					"stun:stun1.l.google.com:19302",
				},
			},
		},
	}
)

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
	}
	// Start cleanup goroutine
	go sm.cleanupExpiredSessions()
	return sm
}

// CreateSession creates a new vault session
func (sm *SessionManager) CreateSession() string {
	sessionID := generateSessionID()
	sm.mu.Lock()
	sm.sessions[sessionID] = &Session{
		ID:        sessionID,
		CreatedAt: time.Now(),
	}
	sm.mu.Unlock()
	log.Printf("✨ Created session: %s", sessionID)
	return sessionID
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	session, exists := sm.sessions[sessionID]
	return session, exists
}

// AddPeer adds a peer to a session
func (sm *SessionManager) AddPeer(sessionID string, peer *Peer) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return &ErrorResponse{Message: "Session not found"}
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if peer.Role == "sender" {
		if session.Sender != nil {
			return &ErrorResponse{Message: "Sender already connected"}
		}
		session.Sender = peer
		log.Printf("📤 Sender joined session: %s", sessionID)
	} else {
		if session.Receiver != nil {
			return &ErrorResponse{Message: "Receiver already connected"}
		}
		session.Receiver = peer
		log.Printf("📥 Receiver joined session: %s", sessionID)
	}

	return nil
}

// RemovePeer removes a peer from a session
func (sm *SessionManager) RemovePeer(sessionID string, peerID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Sender != nil && session.Sender.ID == peerID {
		if session.Sender.PeerConnection != nil {
			session.Sender.PeerConnection.Close()
		}
		session.Sender = nil
		log.Printf("📤 Sender left session: %s", sessionID)
	}
	if session.Receiver != nil && session.Receiver.ID == peerID {
		if session.Receiver.PeerConnection != nil {
			session.Receiver.PeerConnection.Close()
		}
		session.Receiver = nil
		log.Printf("📥 Receiver left session: %s", sessionID)
	}

	// Clean up session if both peers are gone
	if session.Sender == nil && session.Receiver == nil {
		delete(sm.sessions, sessionID)
		log.Printf("🧹 Session cleaned up: %s", sessionID)
	}
}

// BroadcastToSession sends a message to the other peer in a session
func (sm *SessionManager) BroadcastToSession(sessionID, senderPeerID string, msg SignalMessage) error {
	session, exists := sm.GetSession(sessionID)
	if !exists {
		return &ErrorResponse{Message: "Session not found"}
	}

	session.mu.RLock()
	defer session.mu.RUnlock()

	var targetPeer *Peer
	if session.Sender != nil && session.Sender.ID != senderPeerID {
		targetPeer = session.Sender
	} else if session.Receiver != nil && session.Receiver.ID != senderPeerID {
		targetPeer = session.Receiver
	}

	if targetPeer == nil {
		return &ErrorResponse{Message: "Target peer not found"}
	}

	select {
	case targetPeer.SendChan <- msg:
		return nil
	default:
		return &ErrorResponse{Message: "Failed to send message"}
	}
}

// GetPeerPair returns both peers in a session
func (sm *SessionManager) GetPeerPair(sessionID string) (*Peer, *Peer, bool) {
	session, exists := sm.GetSession(sessionID)
	if !exists {
		return nil, nil, false
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	return session.Sender, session.Receiver, true
}

// cleanupExpiredSessions removes sessions older than 30 minutes
func (sm *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for sessionID, session := range sm.sessions {
			if now.Sub(session.CreatedAt) > 30*time.Minute {
				session.mu.Lock()
				// Notify peers of expiration and close peer connections
				if session.Sender != nil {
					if session.Sender.PeerConnection != nil {
						session.Sender.PeerConnection.Close()
					}
					session.Sender.SendChan <- SignalMessage{Type: MsgTypeExpired}
				}
				if session.Receiver != nil {
					if session.Receiver.PeerConnection != nil {
						session.Receiver.PeerConnection.Close()
					}
					session.Receiver.SendChan <- SignalMessage{Type: MsgTypeExpired}
				}
				session.mu.Unlock()
				delete(sm.sessions, sessionID)
				log.Printf("⏰ Expired session: %s", sessionID)
			}
		}
		sm.mu.Unlock()
	}
}

// ErrorResponse represents an error message
type ErrorResponse struct {
	Message string `json:"message"`
}

func (e *ErrorResponse) Error() string {
	return e.Message
}

// generateSessionID creates a random session ID
func generateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generatePeerID creates a random peer ID
func generatePeerID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// createPeerConnection creates a new Pion WebRTC peer connection
func createPeerConnection(peer *Peer) (*webrtc.PeerConnection, error) {
	// Create a new RTCPeerConnection using Pion
	peerConnection, err := webrtc.NewPeerConnection(webrtcConfig)
	if err != nil {
		return nil, err
	}

	// Set up ICE connection state handler
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Printf("🔗 Peer %s ICE Connection State: %s", peer.ID, connectionState.String())

		if connectionState == webrtc.ICEConnectionStateFailed ||
			connectionState == webrtc.ICEConnectionStateDisconnected {
			log.Printf("❌ ICE connection failed/disconnected for peer: %s", peer.ID)
		}
	})

	// Set up connection state handler
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("🔌 Peer %s Connection State: %s", peer.ID, state.String())

		if state == webrtc.PeerConnectionStateConnected {
			log.Printf("✅ Peer %s fully connected!", peer.ID)
		} else if state == webrtc.PeerConnectionStateFailed {
			log.Printf("❌ Peer %s connection failed", peer.ID)
		}
	})

	// Handle ICE candidates - send them to the other peer via signaling
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}

		candidateJSON := candidate.ToJSON()
		payload, err := json.Marshal(candidateJSON)
		if err != nil {
			log.Printf("Error marshaling ICE candidate: %v", err)
			return
		}

		// Send ICE candidate to the other peer
		msg := SignalMessage{
			Type:      MsgTypeICE,
			SessionID: peer.SessionID,
			Payload:   json.RawMessage(payload),
		}

		err = sessionManager.BroadcastToSession(peer.SessionID, peer.ID, msg)
		if err != nil {
			log.Printf("Error sending ICE candidate: %v", err)
		}
	})

	return peerConnection, nil
}

// handleWebSocket handles WebSocket connections
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	peerID := generatePeerID()
	peer := &Peer{
		ID:       peerID,
		Conn:     conn,
		SendChan: make(chan SignalMessage, 10),
	}

	defer func() {
		if peer.PeerConnection != nil {
			peer.PeerConnection.Close()
		}
		close(peer.SendChan)
		conn.Close()
		log.Printf("👋 Peer disconnected: %s", peerID)
		if peer.SessionID != "" {
			sessionManager.RemovePeer(peer.SessionID, peerID)
		}
	}()

	// Start writer goroutine
	go peerWriter(peer)

	// Read messages from client
	for {
		var msg SignalMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("Read error from peer %s: %v", peerID, err)
			break
		}

		handleSignalMessage(peer, msg)
	}
}

// peerWriter sends messages to the peer
func peerWriter(peer *Peer) {
	for msg := range peer.SendChan {
		peer.mu.Lock()
		err := peer.Conn.WriteJSON(msg)
		peer.mu.Unlock()
		if err != nil {
			log.Printf("Write error to peer %s: %v", peer.ID, err)
			return
		}
	}
}

// handleSignalMessage processes incoming signaling messages
func handleSignalMessage(peer *Peer, msg SignalMessage) {
	switch msg.Type {
	case MsgTypeJoin:
		handleJoin(peer, msg)
	case MsgTypeOffer:
		handleOffer(peer, msg)
	case MsgTypeAnswer:
		handleAnswer(peer, msg)
	case MsgTypeICE:
		handleICE(peer, msg)
	default:
		log.Printf("Unknown message type: %s", msg.Type)
	}
}

// handleJoin handles join requests
func handleJoin(peer *Peer, msg SignalMessage) {
	var payload struct {
		SessionID string `json:"sessionId"`
		Role      string `json:"role"`
	}

	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		sendError(peer, "Invalid join payload")
		return
	}

	peer.Role = payload.Role
	sessionID := payload.SessionID

	// If sender and no sessionID, create new session
	if peer.Role == "sender" && sessionID == "" {
		sessionID = sessionManager.CreateSession()
	}

	peer.SessionID = sessionID

	// Create Pion peer connection for this peer
	peerConnection, err := createPeerConnection(peer)
	if err != nil {
		sendError(peer, "Failed to create peer connection: "+err.Error())
		return
	}
	peer.PeerConnection = peerConnection

	// Add peer to session
	err = sessionManager.AddPeer(sessionID, peer)
	if err != nil {
		peerConnection.Close()
		sendError(peer, err.Error())
		return
	}

	// Send ready message with session info
	peer.SendChan <- SignalMessage{
		Type:      MsgTypeReady,
		SessionID: sessionID,
		PeerID:    peer.ID,
	}

	// Check if both peers are connected
	session, _ := sessionManager.GetSession(sessionID)
	session.mu.RLock()
	bothConnected := session.Sender != nil && session.Receiver != nil
	var senderPeer *Peer
	if session.Sender != nil {
		senderPeer = session.Sender
	}
	session.mu.RUnlock()

	if bothConnected {
		log.Printf("🎉 Both peers connected to session: %s", sessionID)
		// Notify sender that receiver has joined - sender should initiate WebRTC offer
		if peer.Role == "receiver" && senderPeer != nil {
			log.Printf("📡 Notifying sender that receiver joined session: %s", sessionID)
			senderPeer.SendChan <- SignalMessage{
				Type:      MsgTypePeerJoined,
				SessionID: sessionID,
			}
		}
	}
}

// handleOffer handles WebRTC offer from sender (relay to receiver)
func handleOffer(peer *Peer, msg SignalMessage) {
	if msg.SessionID == "" {
		sendError(peer, "Session ID required")
		return
	}

	log.Printf("📡 Processing offer in session %s from peer %s", msg.SessionID, peer.ID)

	// Parse the offer SDP
	var offerSDP webrtc.SessionDescription
	if err := json.Unmarshal(msg.Payload, &offerSDP); err != nil {
		sendError(peer, "Invalid offer SDP")
		return
	}

	// Set the remote description on the sender's peer connection (for tracking)
	if peer.PeerConnection != nil {
		err := peer.PeerConnection.SetRemoteDescription(offerSDP)
		if err != nil {
			log.Printf("Warning: Could not set remote description on sender: %v", err)
		}
	}

	// Relay the offer to the receiver
	err := sessionManager.BroadcastToSession(msg.SessionID, peer.ID, msg)
	if err != nil {
		log.Printf("Relay error: %s", err.Error())
		sendError(peer, err.Error())
	}
}

// handleAnswer handles WebRTC answer from receiver (relay to sender)
func handleAnswer(peer *Peer, msg SignalMessage) {
	if msg.SessionID == "" {
		sendError(peer, "Session ID required")
		return
	}

	log.Printf("📡 Processing answer in session %s from peer %s", msg.SessionID, peer.ID)

	// Parse the answer SDP
	var answerSDP webrtc.SessionDescription
	if err := json.Unmarshal(msg.Payload, &answerSDP); err != nil {
		sendError(peer, "Invalid answer SDP")
		return
	}

	// Set the remote description on the receiver's peer connection (for tracking)
	if peer.PeerConnection != nil {
		err := peer.PeerConnection.SetRemoteDescription(answerSDP)
		if err != nil {
			log.Printf("Warning: Could not set remote description on receiver: %v", err)
		}
	}

	// Relay the answer to the sender
	err := sessionManager.BroadcastToSession(msg.SessionID, peer.ID, msg)
	if err != nil {
		log.Printf("Relay error: %s", err.Error())
		sendError(peer, err.Error())
	}
}

// handleICE handles ICE candidates
func handleICE(peer *Peer, msg SignalMessage) {
	if msg.SessionID == "" {
		sendError(peer, "Session ID required")
		return
	}

	log.Printf("🧊 Processing ICE candidate in session %s from peer %s", msg.SessionID, peer.ID)

	// Parse the ICE candidate
	var iceCandidate webrtc.ICECandidateInit
	if err := json.Unmarshal(msg.Payload, &iceCandidate); err != nil {
		sendError(peer, "Invalid ICE candidate")
		return
	}

	// Add ICE candidate to the peer's connection (for server-side tracking)
	if peer.PeerConnection != nil && peer.PeerConnection.RemoteDescription() != nil {
		err := peer.PeerConnection.AddICECandidate(iceCandidate)
		if err != nil {
			log.Printf("Warning: Could not add ICE candidate: %v", err)
		}
	}

	// Relay ICE candidate to the other peer
	err := sessionManager.BroadcastToSession(msg.SessionID, peer.ID, msg)
	if err != nil {
		log.Printf("ICE relay error: %s", err.Error())
		sendError(peer, err.Error())
	}
}

// sendError sends an error message to a peer
func sendError(peer *Peer, message string) {
	errorPayload, _ := json.Marshal(map[string]string{"message": message})
	peer.SendChan <- SignalMessage{
		Type:    MsgTypeError,
		Payload: json.RawMessage(errorPayload),
	}
}

// handleCreateSession creates a new session via HTTP
func handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := sessionManager.CreateSession()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"sessionId": sessionID,
	})
}

// handleHealth returns server health status with Pion version info
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"server":  "Pion WebRTC Signaling Server",
		"version": "1.0.0",
		"webrtc":  "pion/webrtc v4",
	})
}

func main() {
	sessionManager = NewSessionManager()

	// Serve static files
	fs := http.FileServer(http.Dir("../frontend"))
	http.Handle("/", fs)

	// API endpoints
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/api/session", handleCreateSession)
	http.HandleFunc("/api/health", handleHealth)

	port := ":8080"
	log.Println("╔════════════════════════════════════════════════════════════╗")
	log.Println("║        🔐 Secure P2P File Vault (Pion WebRTC)              ║")
	log.Println("╠════════════════════════════════════════════════════════════╣")
	log.Printf("║  🌐 Server:    http://localhost%s                        ║", port)
	log.Println("║  📡 WebSocket: ws://localhost:8080/ws                      ║")
	log.Println("║  📁 Frontend:  ../frontend                                 ║")
	log.Println("║  🔗 Powered by: pion/webrtc v4                             ║")
	log.Println("╚════════════════════════════════════════════════════════════╝")

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Server error:", err)
	}
}
