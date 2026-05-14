package services

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// WebSocketIdempotency handles duplicate event detection
type WebSocketIdempotency struct {
	redis *redis.Client
}

// EventMessage with idempotency key
type EventMessage struct {
	EventID   string                 `json:"event_id"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp int64                  `json:"timestamp"`
	UserID    string                 `json:"user_id"`
}

func NewWebSocketIdempotency(redis *redis.Client) *WebSocketIdempotency {
	return &WebSocketIdempotency{redis: redis}
}

// IsDuplicate checks if event was already processed
func (w *WebSocketIdempotency) IsDuplicate(userID, eventID string) bool {
	key := "ws:event:" + userID + ":" + eventID
	exists, _ := w.redis.Exists(context.Background(), key).Result()
	return exists > 0
}

// MarkProcessed stores event_id to prevent duplicates
func (w *WebSocketIdempotency) MarkProcessed(userID, eventID string) error {
	key := "ws:event:" + userID + ":" + eventID
	return w.redis.Set(context.Background(), key, "1", 24*time.Hour).Err()
}

// ProcessEvent handles idempotent event processing
func (w *WebSocketIdempotency) ProcessEvent(msg EventMessage, handler func(EventMessage) error) error {
	// Check for duplicate
	if w.IsDuplicate(msg.UserID, msg.EventID) {
		return nil // Silently ignore duplicate
	}

	// Process event
	if err := handler(msg); err != nil {
		return err
	}

	// Mark as processed
	return w.MarkProcessed(msg.UserID, msg.EventID)
}

// GetLastEventID retrieves last processed event for reconnection sync
func (w *WebSocketIdempotency) GetLastEventID(userID string) (string, error) {
	key := "ws:last_event:" + userID
	return w.redis.Get(context.Background(), key).Result()
}

// SetLastEventID updates last processed event
func (w *WebSocketIdempotency) SetLastEventID(userID, eventID string) error {
	key := "ws:last_event:" + userID
	return w.redis.Set(context.Background(), key, eventID, 24*time.Hour).Err()
}

// SyncEvents retrieves missed events after reconnection
func (w *WebSocketIdempotency) SyncEvents(userID string, lastEventID string) ([]EventMessage, error) {
	// In production: Query event store for events after lastEventID
	// For now: return empty (client should handle missing events gracefully)
	return []EventMessage{}, nil
}

// WebSocketIdempotencyConfig returns configuration
func GetWebSocketIdempotencyConfig() map[string]interface{} {
	return map[string]interface{}{
		"event_ttl_hours":      24,
		"max_events_per_user":  1000,
		"dedup_window_seconds": 300,
		"sync_batch_size":      50,
	}
}
