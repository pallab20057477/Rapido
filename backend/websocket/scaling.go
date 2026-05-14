package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// WebSocketScaling provides multi-server WebSocket scaling
type WebSocketScaling struct {
	serverID    string
	redis       *redis.Client
	ctx         context.Context
	cancel      context.CancelFunc
}

// ServerPresence tracks WebSocket server instances
type ServerPresence struct {
	ServerID   string    `json:"server_id"`
	Clients    int       `json:"clients"`
	Rides      []string  `json:"rides"`
	LastPing   time.Time `json:"last_ping"`
	StartedAt  time.Time `json:"started_at"`
}

// NewWebSocketScaling creates scaling manager
func NewWebSocketScaling(serverID string) *WebSocketScaling {
	ctx, cancel := context.WithCancel(context.Background())
	return &WebSocketScaling{
		serverID: serverID,
		redis:    database.RedisClient,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start starts the scaling manager
func (ws *WebSocketScaling) Start(handler *WebSocketHandler) {
	// Register this server
	ws.registerServer()
	
	// Start presence heartbeat
	go ws.presenceHeartbeat(handler)
	
	// Subscribe to inter-server messages
	go ws.subscribeToInterServer(handler)
	
	// Start client count reporter
	go ws.reportClientCount(handler)
	
	utils.Info("WebSocket scaling started",
		zap.String("server_id", ws.serverID))
}

// registerServer registers this WebSocket server in Redis
func (ws *WebSocketScaling) registerServer() {
	presence := ServerPresence{
		ServerID:  ws.serverID,
		Clients:   0,
		Rides:     []string{},
		LastPing:  time.Now(),
		StartedAt: time.Now(),
	}
	
	data, _ := json.Marshal(presence)
	key := fmt.Sprintf("ws:server:%s", ws.serverID)
	
	ws.redis.Set(ws.ctx, key, data, 30*time.Second)
	ws.redis.SAdd(ws.ctx, "ws:servers", ws.serverID)
}

// presenceHeartbeat updates server presence every 10 seconds
func (ws *WebSocketScaling) presenceHeartbeat(handler *WebSocketHandler) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ws.ctx.Done():
			return
		case <-ticker.C:
			ws.updatePresence(handler)
		}
	}
}

// updatePresence updates server presence in Redis
func (ws *WebSocketScaling) updatePresence(handler *WebSocketHandler) {
	handler.mu.RLock()
	clientCount := len(handler.clients)
	
	// Collect active rides
	rideMap := make(map[string]bool)
	for _, client := range handler.clients {
		if client.rideID != "" {
			rideMap[client.rideID] = true
		}
	}
	handler.mu.RUnlock()
	
	rides := make([]string, 0, len(rideMap))
	for rideID := range rideMap {
		rides = append(rides, rideID)
	}
	
	presence := ServerPresence{
		ServerID: ws.serverID,
		Clients:  clientCount,
		Rides:    rides,
		LastPing: time.Now(),
	}
	
	data, _ := json.Marshal(presence)
	key := fmt.Sprintf("ws:server:%s", ws.serverID)
	
	ws.redis.Set(ws.ctx, key, data, 30*time.Second)
}

// subscribeToInterServer subscribes to inter-server messages
func (ws *WebSocketScaling) subscribeToInterServer(handler *WebSocketHandler) {
	// Subscribe to direct messages for this server
	channel := fmt.Sprintf("ws:server:%s:messages", ws.serverID)
	pubsub := ws.redis.Subscribe(ws.ctx, channel)
	defer pubsub.Close()
	
	ch := pubsub.Channel()
	for msg := range ch {
		if msg == nil {
			continue
		}
		
		var message WebSocketMessage
		if err := json.Unmarshal([]byte(msg.Payload), &message); err != nil {
			continue
		}
		
		// Deliver to local clients
		handler.broadcast <- message
	}
}

// reportClientCount reports client count for monitoring
func (ws *WebSocketScaling) reportClientCount(handler *WebSocketHandler) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ws.ctx.Done():
			return
		case <-ticker.C:
			handler.mu.RLock()
			count := len(handler.clients)
			handler.mu.RUnlock()
			
			// Publish to metrics channel
			ws.redis.Publish(ws.ctx, "ws:metrics:clients", fmt.Sprintf("%s:%d", ws.serverID, count))
		}
	}
}

// GetServerStats returns stats for all WebSocket servers
func (ws *WebSocketScaling) GetServerStats() ([]ServerPresence, error) {
	// Get all server IDs
	serverIDs, err := ws.redis.SMembers(ws.ctx, "ws:servers").Result()
	if err != nil {
		return nil, err
	}
	
	var servers []ServerPresence
	for _, serverID := range serverIDs {
		key := fmt.Sprintf("ws:server:%s", serverID)
		data, err := ws.redis.Get(ws.ctx, key).Result()
		if err != nil {
			continue // Server might be down
		}
		
		var presence ServerPresence
		if err := json.Unmarshal([]byte(data), &presence); err != nil {
			continue
		}
		
		// Check if server is still alive (last ping within 60 seconds)
		if time.Since(presence.LastPing) > 60*time.Second {
			// Remove dead server
			ws.redis.SRem(ws.ctx, "ws:servers", serverID)
			ws.redis.Del(ws.ctx, key)
			continue
		}
		
		servers = append(servers, presence)
	}
	
	return servers, nil
}

// GetTotalClients returns total clients across all servers
func (ws *WebSocketScaling) GetTotalClients() int {
	servers, err := ws.GetServerStats()
	if err != nil {
		return 0
	}
	
	total := 0
	for _, server := range servers {
		total += server.Clients
	}
	return total
}

// FindServerForUser finds which server has a specific user connected
func (ws *WebSocketScaling) FindServerForUser(userID string) (string, error) {
	servers, err := ws.GetServerStats()
	if err != nil {
		return "", err
	}
	
	// Check each server's user list in Redis
	for _, server := range servers {
		key := fmt.Sprintf("ws:server:%s:users", server.ServerID)
		exists, _ := ws.redis.SIsMember(ws.ctx, key, userID).Result()
		if exists {
			return server.ServerID, nil
		}
	}
	
	return "", nil
}

// SendToUserOnAnyServer sends message to a user regardless of which server they're on
func (ws *WebSocketScaling) SendToUserOnAnyServer(userID string, message WebSocketMessage) error {
	// First try local delivery
	if handler := GetHandler(); handler != nil {
		handler.mu.RLock()
		client, exists := handler.clients[userID]
		handler.mu.RUnlock()
		
		if exists {
			select {
			case client.send <- message:
				return nil
			default:
				// Channel full, try fanout
			}
		}
	}
	
	// Find which server has this user
	serverID, err := ws.FindServerForUser(userID)
	if err != nil {
		return err
	}
	
	if serverID == "" {
		// User not connected anywhere
		return fmt.Errorf("user %s not connected", userID)
	}
	
	// Send to that server's channel
	channel := fmt.Sprintf("ws:server:%s:messages", serverID)
	data, _ := json.Marshal(message)
	
	return ws.redis.Publish(ws.ctx, channel, data).Err()
}

// GracefulShutdown handles graceful shutdown
func (ws *WebSocketScaling) GracefulShutdown() {
	ws.cancel()
	
	// Remove this server from the set
	ws.redis.SRem(ws.ctx, "ws:servers", ws.serverID)
	ws.redis.Del(ws.ctx, fmt.Sprintf("ws:server:%s", ws.serverID))
	ws.redis.Del(ws.ctx, fmt.Sprintf("ws:server:%s:users", ws.serverID))
	
	utils.Info("WebSocket scaling shutdown complete",
		zap.String("server_id", ws.serverID))
}

// Global scaling instance
var Scaling *WebSocketScaling

// InitWebSocketScaling initializes the scaling manager
func InitWebSocketScaling(serverID string) {
	Scaling = NewWebSocketScaling(serverID)
	Scaling.Start(GetHandler())
}
