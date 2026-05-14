package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisPubSubManager struct {
	redis       *redis.Client
	pubsub      *redis.PubSub
	ctx         context.Context
	cancel      context.CancelFunc
	connections map[string]*Client
	mu          sync.RWMutex
}

type PubSubMessage struct {
	Type      string      `json:"type"`
	Channel   string      `json:"channel"`
	Room      string      `json:"room,omitempty"`
	UserID    string      `json:"user_id,omitempty"`
	DriverID  string      `json:"driver_id,omitempty"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

type RoomManager struct {
	rooms map[string]map[string]bool // room -> connection IDs
	mu    sync.RWMutex
}

func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: make(map[string]map[string]bool),
	}
}

func (rm *RoomManager) JoinRoom(connectionID, room string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.rooms[room] == nil {
		rm.rooms[room] = make(map[string]bool)
	}
	rm.rooms[room][connectionID] = true
}

func (rm *RoomManager) LeaveRoom(connectionID, room string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.rooms[room] != nil {
		delete(rm.rooms[room], connectionID)
		if len(rm.rooms[room]) == 0 {
			delete(rm.rooms, room)
		}
	}
}

func (rm *RoomManager) GetRoomMembers(room string) []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	members := make([]string, 0, len(rm.rooms[room]))
	for memberID := range rm.rooms[room] {
		members = append(members, memberID)
	}
	return members
}

func NewRedisPubSubManager(redisClient *redis.Client) *RedisPubSubManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &RedisPubSubManager{
		redis:       redisClient,
		ctx:         ctx,
		cancel:      cancel,
		connections: make(map[string]*Client),
	}
}

func (rpm *RedisPubSubManager) Start() error {
	// Subscribe to WebSocket channels
	rpm.pubsub = rpm.redis.Subscribe(rpm.ctx, "websocket:*")

	// Start listening for messages
	go rpm.listenForMessages()

	log.Println("Redis Pub/Sub manager started")
	return nil
}

func (rpm *RedisPubSubManager) Stop() {
	if rpm.cancel != nil {
		rpm.cancel()
	}

	if rpm.pubsub != nil {
		rpm.pubsub.Close()
	}

	log.Println("Redis Pub/Sub manager stopped")
}

func (rpm *RedisPubSubManager) listenForMessages() {
	ch := rpm.pubsub.Channel()

	for {
		select {
		case <-rpm.ctx.Done():
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}

			var pubMsg PubSubMessage
			if err := json.Unmarshal([]byte(msg.Payload), &pubMsg); err != nil {
				log.Printf("Failed to unmarshal PubSub message: %v", err)
				continue
			}

			rpm.handleIncomingMessage(pubMsg)
		}
	}
}

func (rpm *RedisPubSubManager) handleIncomingMessage(msg PubSubMessage) {
	rpm.mu.RLock()
	defer rpm.mu.RUnlock()

	switch msg.Type {
	case "broadcast_to_room":
		rpm.sendToRoom(msg.Room, msg.Data)
	case "send_to_user":
		rpm.sendToUser(msg.UserID, msg.Data)
	case "send_to_driver":
		rpm.sendToDriver(msg.DriverID, msg.Data)
	case "broadcast_to_all":
		rpm.broadcastToAll(msg.Data)
	}
}

func (rpm *RedisPubSubManager) sendToRoom(room string, data interface{}) {
	// This would be implemented by the WebSocket handler
	// that has access to the actual connections
	log.Printf("Broadcasting to room %s: %v", room, data)
}

func (rpm *RedisPubSubManager) sendToUser(userID string, data interface{}) {
	log.Printf("Sending to user %s: %v", userID, data)
}

func (rpm *RedisPubSubManager) sendToDriver(driverID string, data interface{}) {
	log.Printf("Sending to driver %s: %v", driverID, data)
}

func (rpm *RedisPubSubManager) broadcastToAll(data interface{}) {
	log.Printf("Broadcasting to all: %v", data)
}

// Publish methods for sending messages through Redis Pub/Sub
func (rpm *RedisPubSubManager) PublishToRoom(room string, data interface{}) error {
	msg := PubSubMessage{
		Type:      "broadcast_to_room",
		Channel:   "websocket:broadcast",
		Room:      room,
		Data:      data,
		Timestamp: time.Now(),
	}

	return rpm.publishMessage(msg)
}

func (rpm *RedisPubSubManager) PublishToUser(userID string, data interface{}) error {
	msg := PubSubMessage{
		Type:      "send_to_user",
		Channel:   "websocket:user",
		UserID:    userID,
		Data:      data,
		Timestamp: time.Now(),
	}

	return rpm.publishMessage(msg)
}

func (rpm *RedisPubSubManager) PublishToDriver(driverID string, data interface{}) error {
	msg := PubSubMessage{
		Type:      "send_to_driver",
		Channel:   "websocket:driver",
		DriverID:  driverID,
		Data:      data,
		Timestamp: time.Now(),
	}

	return rpm.publishMessage(msg)
}

func (rpm *RedisPubSubManager) PublishToAll(data interface{}) error {
	msg := PubSubMessage{
		Type:      "broadcast_to_all",
		Channel:   "websocket:all",
		Data:      data,
		Timestamp: time.Now(),
	}

	return rpm.publishMessage(msg)
}

func (rpm *RedisPubSubManager) publishMessage(msg PubSubMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal PubSub message: %w", err)
	}

	return rpm.redis.Publish(rpm.ctx, msg.Channel, data).Err()
}

// Room-based messaging for different use cases
func (rpm *RedisPubSubManager) JoinRideRoom(connectionID, rideID string) {
	room := fmt.Sprintf("ride:%s", rideID)
	// This would be handled by the WebSocket handler
	log.Printf("Connection %s joined ride room %s", connectionID, room)
}

func (rpm *RedisPubSubManager) LeaveRideRoom(connectionID, rideID string) {
	room := fmt.Sprintf("ride:%s", rideID)
	// This would be handled by the WebSocket handler
	log.Printf("Connection %s left ride room %s", connectionID, room)
}

func (rpm *RedisPubSubManager) PublishToRide(rideID string, data interface{}) error {
	room := fmt.Sprintf("ride:%s", rideID)
	return rpm.PublishToRoom(room, data)
}

func (rpm *RedisPubSubManager) PublishToDriverLocation(driverID string, location map[string]interface{}) error {
	data := map[string]interface{}{
		"type":      "location_update",
		"driver_id": driverID,
		"location":  location,
		"timestamp": time.Now().Unix(),
	}

	return rpm.PublishToDriver(driverID, data)
}

func (rpm *RedisPubSubManager) PublishRideStatusUpdate(rideID string, status string, data interface{}) error {
	updateData := map[string]interface{}{
		"type":      "ride_status_update",
		"ride_id":   rideID,
		"status":    status,
		"data":      data,
		"timestamp": time.Now().Unix(),
	}

	return rpm.PublishToRide(rideID, updateData)
}

// Health check for Redis Pub/Sub
func (rpm *RedisPubSubManager) HealthCheck() error {
	if rpm.redis == nil {
		return fmt.Errorf("Redis client is nil")
	}

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rpm.redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis ping failed: %w", err)
	}

	return nil
}

// Get statistics for monitoring
func (rpm *RedisPubSubManager) GetStats() map[string]interface{} {
	rpm.mu.RLock()
	defer rpm.mu.RUnlock()

	return map[string]interface{}{
		"active_connections": len(rpm.connections),
		"pubsub_active":      rpm.pubsub != nil,
		"redis_connected":    rpm.redis != nil,
	}
}
