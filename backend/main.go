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
)

// Message types for WebSocket signaling
const (
	MsgTypeJoin    = "join"
	MsgTypeOffer   = "offer"
	MsgTypeAnswer  = "answer"
	MsgTypeICE     = "ice"
	MsgTypeReady   = "ready"
	MsgTypeError   = "error"
	MsgTypeExpired = "expired"
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
	ID       string
	Conn     *websocket.Conn
	Role     string // "sender" or "receiver"
	SendChan chan SignalMessage
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
	sessionManager = &SessionManager{
		sessions: make(map[string]*Session),
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
	log.Printf("Created session: %s", sessionID)
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
		log.Printf("Sender joined session: %s", sessionID)
	} else {
		if session.Receiver != nil {
			return &ErrorResponse{Message: "Receiver already connected"}
		}
		session.Receiver = peer
		log.Printf("Receiver joined session: %s", sessionID)
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
		session.Sender = nil
		log.Printf("Sender left session: %s", sessionID)
	}
	if session.Receiver != nil && session.Receiver.ID == peerID {
		session.Receiver = nil
		log.Printf("Receiver left session: %s", sessionID)
	}

	// Clean up session if both peers are gone
	if session.Sender == nil && session.Receiver == nil {
		delete(sm.sessions, sessionID)
		log.Printf("Session cleaned up: %s", sessionID)
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
				// Notify peers of expiration
				if session.Sender != nil {
					session.Sender.SendChan <- SignalMessage{Type: MsgTypeExpired}
				}
				if session.Receiver != nil {
					session.Receiver.SendChan <- SignalMessage{Type: MsgTypeExpired}
				}
				session.mu.Unlock()
				delete(sm.sessions, sessionID)
				log.Printf("Expired session: %s", sessionID)
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
		close(peer.SendChan)
		conn.Close()
		log.Printf("Peer disconnected: %s", peerID)
	}()

	// Start writer goroutine
	go peerWriter(peer)

	// Read messages from client
	for {
		var msg SignalMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("Read error from peer %s: %v", peerID, err)
			// Clean up peer from any session
			if msg.SessionID != "" {
				sessionManager.RemovePeer(msg.SessionID, peerID)
			}
			break
		}

		handleSignalMessage(peer, msg)
	}
}

// peerWriter sends messages to the peer
func peerWriter(peer *Peer) {
	for msg := range peer.SendChan {
		err := peer.Conn.WriteJSON(msg)
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
	case MsgTypeOffer, MsgTypeAnswer, MsgTypeICE:
		handleRelay(peer, msg)
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

	// Add peer to session
	err := sessionManager.AddPeer(sessionID, peer)
	if err != nil {
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
	session.mu.RUnlock()

	if bothConnected {
		log.Printf("Both peers connected to session: %s", sessionID)
	}
}

// handleRelay relays signaling messages between peers
func handleRelay(peer *Peer, msg SignalMessage) {
	if msg.SessionID == "" {
		sendError(peer, "Session ID required")
		return
	}

	err := sessionManager.BroadcastToSession(msg.SessionID, peer.ID, msg)
	if err != nil {
		sendError(peer, err.Error())
	}
}

// sendError sends an error message to a peer
func sendError(peer *Peer, message string) {
	peer.SendChan <- SignalMessage{
		Type:    MsgTypeError,
		Payload: json.RawMessage(`{"message":"` + message + `"}`),
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

func main() {
	sessionManager = NewSessionManager()

	// Serve static files
	fs := http.FileServer(http.Dir("../frontend"))
	http.Handle("/", fs)

	// API endpoints
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/api/session", handleCreateSession)

	port := ":8080"
	log.Printf("🔐 Secure P2P File Vault server starting on http://localhost%s", port)
	log.Printf("📁 Serving frontend from ../frontend")

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Server error:", err)
	}
}
