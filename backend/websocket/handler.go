package websocket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

// Message types for WebSocket communication
const (
	MessageTypeLocationUpdate = "location_update"
	MessageTypeRideStatus     = "ride_status"
	MessageTypeDriverLocation = "driver_location"
	MessageTypeChat           = "chat"
	MessageTypeSOS            = "sos"
	MessageTypePing           = "ping"
	MessageTypePong           = "pong"
	MessageTypeSubscribe      = "subscribe"
	MessageTypeUnsubscribe    = "unsubscribe"
)

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	RideID    string          `json:"ride_id,omitempty"`
	UserID    string          `json:"user_id,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// LocationPayload represents location update payload
type LocationPayload struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
	Accuracy  float64 `json:"accuracy,omitempty"`
	Speed     float64 `json:"speed,omitempty"`
	Heading   float64 `json:"heading,omitempty"`
	Timestamp int64   `json:"timestamp"`
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	upgrader   websocket.Upgrader
	clients    map[string]*Client
	broadcast  chan WebSocketMessage
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	serverID   string
	pubsub     *redis.PubSub
}

var (
	defaultHandler *WebSocketHandler
	handlerOnce    sync.Once
)

// Client represents a WebSocket client
type Client struct {
	conn     *websocket.Conn
	send     chan WebSocketMessage
	userID   string
	userType string // rider, driver
	rideID   string // Current ride being tracked
}

type fanoutMessage struct {
	Target    string          `json:"target"`
	TargetID  string          `json:"target_id,omitempty"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	Source    string          `json:"source"`
	Timestamp int64           `json:"timestamp"`
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler() *WebSocketHandler {
	allowedOrigins := parseAllowedWebSocketOrigins(utils.GetEnv("CORS_ALLOW_ORIGIN", ""))
	return &WebSocketHandler{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := strings.TrimSpace(r.Header.Get("Origin"))
				if origin == "" {
					// Allow empty origin only in development (e.g., raw socket clients, localhost file://)
					// Production requires explicit Origin header matching allowlist
					env := strings.ToLower(strings.TrimSpace(utils.GetEnv("APP_ENV", "production")))
					if env != "development" && env != "dev" {
						return false
					}
					return true
				}
				if len(allowedOrigins) == 0 {
					return false
				}
				for _, allowed := range allowedOrigins {
					if allowed == "*" || strings.EqualFold(origin, allowed) {
						return true
					}
				}
				return false
			},
		},
		clients:    make(map[string]*Client),
		broadcast:  make(chan WebSocketMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		serverID:   fmt.Sprintf("ws-%d", time.Now().UnixNano()),
	}
}

func parseAllowedWebSocketOrigins(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

// GetHandler returns the shared WebSocket handler singleton.
func GetHandler() *WebSocketHandler {
	handlerOnce.Do(func() {
		defaultHandler = NewWebSocketHandler()
		go defaultHandler.Run()
		defaultHandler.initPubSub()
	})
	return defaultHandler
}

func (h *WebSocketHandler) initPubSub() {
	if database.RedisClient == nil {
		return
	}

	h.pubsub = database.SubscribeToChannel("ws:events")
	go func() {
		ch := h.pubsub.Channel()
		for msg := range ch {
			if msg == nil {
				continue
			}

			var incoming fanoutMessage
			if err := json.Unmarshal([]byte(msg.Payload), &incoming); err != nil {
				continue
			}

			if incoming.Source == h.serverID {
				continue
			}

			switch incoming.Target {
			case "user":
				h.sendToUserLocal(incoming.TargetID, incoming.EventType, incoming.Payload)
			case "ride":
				h.sendToRideLocal(incoming.TargetID, incoming.EventType, incoming.Payload)
			}
		}
	}()
}

func (h *WebSocketHandler) publishFanout(target, targetID, eventType string, payload json.RawMessage) {
	if database.RedisClient == nil {
		return
	}

	message := fanoutMessage{
		Target:    target,
		TargetID:  targetID,
		EventType: eventType,
		Payload:   payload,
		Source:    h.serverID,
		Timestamp: time.Now().Unix(),
	}

	encoded, err := json.Marshal(message)
	if err != nil {
		return
	}

	_ = database.PublishLocationUpdate("ws:events", encoded)
}

func (h *WebSocketHandler) sendToUserLocal(userID, eventType string, payload json.RawMessage) {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()
	if !ok || client == nil {
		return
	}

	select {
	case client.send <- WebSocketMessage{Type: eventType, Payload: payload, UserID: userID, Timestamp: time.Now().Unix()}:
	default:
	}
}

func (h *WebSocketHandler) sendToRideLocal(rideID, eventType string, payload json.RawMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()

	message := WebSocketMessage{Type: eventType, Payload: payload, RideID: rideID, Timestamp: time.Now().Unix()}
	for id, client := range h.clients {
		if client == nil || client.rideID != rideID {
			continue
		}
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(h.clients, id)
		}
	}
}

// SendToUser sends a message to a connected user if present.
func (h *WebSocketHandler) SendToUser(userID string, data interface{}) error {
	return h.SendToUserEvent(userID, MessageTypeRideStatus, data)
}

// SendToUserEvent sends a typed event to a connected user if present.
func (h *WebSocketHandler) SendToUserEvent(userID, eventType string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	h.sendToUserLocal(userID, eventType, payload)
	h.publishFanout("user", userID, eventType, payload)
	return nil
}

// SendToDriver sends a message to a connected driver if present.
func (h *WebSocketHandler) SendToDriver(driverID string, data interface{}) error {
	return h.SendToUserEvent(driverID, MessageTypeRideStatus, data)
}

// SendRideEvent broadcasts a ride-scoped event to subscribed clients.
func (h *WebSocketHandler) SendRideEvent(rideID, eventType string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	h.sendToRideLocal(rideID, eventType, payload)
	h.publishFanout("ride", rideID, eventType, payload)

	return nil
}

// Run starts the WebSocket hub
func (h *WebSocketHandler) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.userID] = client
			h.mu.Unlock()
			log.Printf("Client registered: %s", client.userID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.userID]; ok {
				delete(h.clients, client.userID)
				close(client.send)
				log.Printf("Client unregistered: %s", client.userID)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			// Send to specific ride participants
			h.mu.RLock()
			for id, client := range h.clients {
				if client.rideID == message.RideID || client.userID == message.UserID {
					select {
					case client.send <- message:
					default:
						close(client.send)
						delete(h.clients, id)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// HandleWebSocket handles WebSocket connections
// Supports unified endpoint: /ws?type=rider|driver|admin&token=xxx&user_id=xxx
// Production: Uses Redis Pub/Sub for multi-node synchronization
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	// Prefer validated middleware context, but keep query param compatibility.
	userType := c.GetString("userRole")
	if userType == "" {
		userType = c.GetString("user_type")
	}
	if userType == "" {
		userType = c.Query("type")
		if userType == "" {
			userType = c.Query("user_type")
		}
	}

	userID := c.GetString("userID")
	if userID == "" {
		userID = c.GetString("user_id")
	}
	if userID == "" {
		userID = c.Query("user_id")
	}

	token := c.GetString("token")
	if token == "" {
		token = c.Query("token")
	}

	// Validate required params
	if userType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type query param required (rider/driver/admin)"})
		return
	}

	// Validate type
	validTypes := map[string]bool{"rider": true, "driver": true, "admin": true}
	if !validTypes[userType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid type. Use: rider, driver, or admin"})
		return
	}

	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
		return
	}

	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	client := &Client{
		conn:     conn,
		send:     make(chan WebSocketMessage, 256),
		userID:   userID,
		userType: userType,
	}

	h.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump(h)
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump(h *WebSocketHandler) {
	defer func() {
		h.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg WebSocketMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		msg.Timestamp = time.Now().Unix()
		msg.UserID = c.userID

		// Handle different message types
		switch msg.Type {
		case MessageTypeLocationUpdate:
			// Broadcast location update to ride participants
			h.broadcast <- msg

		case MessageTypeSubscribe:
			// Subscribe to ride updates
			var payload struct {
				RideID string `json:"ride_id"`
			}
			json.Unmarshal(msg.Payload, &payload)
			c.rideID = payload.RideID

		case MessageTypeUnsubscribe:
			// Unsubscribe from ride
			c.rideID = ""

		case MessageTypePing:
			// Send pong response
			c.send <- WebSocketMessage{
				Type:      MessageTypePong,
				Timestamp: time.Now().Unix(),
			}

		case MessageTypeChat:
			// Broadcast chat message
			h.broadcast <- msg
		}
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// BroadcastLocationUpdate broadcasts driver location to ride participants
func (h *WebSocketHandler) BroadcastLocationUpdate(rideID string, payload LocationPayload) {
	payloadBytes, _ := json.Marshal(payload)
	msg := WebSocketMessage{
		Type:      MessageTypeLocationUpdate,
		Payload:   payloadBytes,
		RideID:    rideID,
		Timestamp: time.Now().Unix(),
	}
	h.broadcast <- msg
}

// BroadcastRideStatus broadcasts ride status update
func (h *WebSocketHandler) BroadcastRideStatus(rideID, status string, data map[string]interface{}) {
	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"status": status,
		"data":   data,
	})
	msg := WebSocketMessage{
		Type:      MessageTypeRideStatus,
		Payload:   payloadBytes,
		RideID:    rideID,
		Timestamp: time.Now().Unix(),
	}
	h.broadcast <- msg
}
