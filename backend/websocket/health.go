package websocket

import (
	"time"

	"rapido-backend/utils"

	"go.uber.org/zap"
)

// ConnectionHealth manages WebSocket connection health checks
type ConnectionHealth struct {
	handler     *WebSocketHandler
	pingInterval time.Duration
	maxIdleTime  time.Duration
}

// NewConnectionHealth creates health checker
func NewConnectionHealth(handler *WebSocketHandler) *ConnectionHealth {
	return &ConnectionHealth{
		handler:      handler,
		pingInterval: 30 * time.Second,
		maxIdleTime:  2 * time.Minute,
	}
}

// Start starts health monitoring
func (ch *ConnectionHealth) Start() {
	// Start ping checker
	go ch.pingChecker()
	
	// Start idle connection cleaner
	go ch.idleConnectionCleaner()
	
	utils.Info("WebSocket health monitoring started",
		zap.Duration("ping_interval", ch.pingInterval),
		zap.Duration("max_idle", ch.maxIdleTime))
}

// pingChecker sends periodic pings to check connection health
func (ch *ConnectionHealth) pingChecker() {
	ticker := time.NewTicker(ch.pingInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		ch.handler.mu.RLock()
		clients := make([]*Client, 0, len(ch.handler.clients))
		for _, client := range ch.handler.clients {
			clients = append(clients, client)
		}
		ch.handler.mu.RUnlock()
		
		// Send ping to all clients
		for _, client := range clients {
			select {
			case client.send <- WebSocketMessage{
				Type:      MessageTypePing,
				Timestamp: time.Now().Unix(),
			}:
				// Ping sent
			default:
				// Channel blocked, connection unhealthy
				ch.handler.unregister <- client
			}
		}
	}
}

// idleConnectionCleaner removes idle connections
func (ch *ConnectionHealth) idleConnectionCleaner() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		ch.handler.mu.RLock()
		clients := make([]*Client, 0, len(ch.handler.clients))
		for _, client := range ch.handler.clients {
			clients = append(clients, client)
		}
		ch.handler.mu.RUnlock()
		
		// Check for idle connections
		for _, client := range clients {
			// Check last activity (would need to track in Client struct)
			// For now, rely on ping/pong
			_ = client
		}
	}
}

// GetHealthStats returns WebSocket health statistics
func (ch *ConnectionHealth) GetHealthStats() map[string]interface{} {
	ch.handler.mu.RLock()
	defer ch.handler.mu.RUnlock()
	
	stats := map[string]interface{}{
		"total_clients":     len(ch.handler.clients),
		"max_idle_time":     ch.maxIdleTime.String(),
		"ping_interval":     ch.pingInterval.String(),
		"timestamp":         time.Now(),
	}
	
	// Count by type
	riders := 0
	drivers := 0
	withRides := 0
	
	for _, client := range ch.handler.clients {
		if client.userType == "rider" {
			riders++
		} else if client.userType == "driver" {
			drivers++
		}
		
		if client.rideID != "" {
			withRides++
		}
	}
	
	stats["riders"] = riders
	stats["drivers"] = drivers
	stats["active_rides"] = withRides
	
	return stats
}

// Global health checker
var HealthChecker *ConnectionHealth

// InitHealthChecker initializes connection health monitoring
func InitHealthChecker() {
	HealthChecker = NewConnectionHealth(GetHandler())
	HealthChecker.Start()
}
