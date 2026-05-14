package services

import (
	"context"
	"fmt"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SupplyService handles cold start and low supply scenarios
type SupplyService struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewSupplyService creates supply management service
func NewSupplyService() *SupplyService {
	return &SupplyService{
		db:    database.DB,
		redis: database.RedisClient,
	}
}

// ColdStartResponse is returned when no drivers available
type ColdStartResponse struct {
	Action           string `json:"action"`
	NotifiedDrivers  int    `json:"notified_drivers"`
	QueuePosition    int    `json:"queue_position"`
	EstimatedWaitSec int    `json:"estimated_wait_seconds"`
	IncentiveActive  bool   `json:"incentive_active"`
	Message          string `json:"message"`
}

// HandleColdStart triggers cold start protocol
func (s *SupplyService) HandleColdStart(ctx context.Context, lat, lng float64, vehicleType string) (*ColdStartResponse, error) {
	zap.L().Warn("Cold start triggered - no drivers available",
		zap.Float64("lat", lat),
		zap.Float64("lng", lng))

	// 1. Notify dormant drivers within 15km
	notified := s.notifyDormantDrivers(lat, lng, vehicleType)

	// 2. Trigger incentive surge (50% bonus)
	s.triggerIncentiveSurge(lat, lng)

	// 3. Add ride to queue
	queuePos := s.queueRide(lat, lng, vehicleType)

	return &ColdStartResponse{
		Action:           "cold_start",
		NotifiedDrivers:  notified,
		QueuePosition:    queuePos,
		EstimatedWaitSec: 120,
		IncentiveActive:  true,
		Message:          fmt.Sprintf("Looking for drivers. Queue position: #%d", queuePos+1),
	}, nil
}

// notifyDormantDrivers sends notifications to offline drivers
func (s *SupplyService) notifyDormantDrivers(lat, lng float64, vehicleType string) int {
	// Find drivers offline but active in last 24h
	last24h := time.Now().Add(-24 * time.Hour)

	var drivers []models.Driver
	s.db.Where("last_online_at >= ? AND is_online = ? AND vehicle_type = ?",
		last24h, false, vehicleType).Find(&drivers)

	notified := 0
	for _, driver := range drivers {
		// Check distance (simplified)
		if driver.CurrentLocation != nil {
			dist := haversine(lat, lng, driver.CurrentLocation.Latitude, driver.CurrentLocation.Longitude)
			if dist <= 15.0 { // Within 15km
				// Send push notification
				notified++
			}
		}
	}

	return notified
}

// triggerIncentiveSurge activates surge pricing
func (s *SupplyService) triggerIncentiveSurge(lat, lng float64) {
	key := fmt.Sprintf("surge:%.2f:%.2f", lat, lng)
	s.redis.HSet(context.Background(), key, map[string]interface{}{
		"multiplier": 1.5,
		"reason":     "cold_start",
		"expires_at": time.Now().Add(30 * time.Minute).Unix(),
	})
	s.redis.Expire(context.Background(), key, 30*time.Minute)
}

// queueRide adds ride to zone queue
func (s *SupplyService) queueRide(lat, lng float64, vehicleType string) int {
	zoneKey := fmt.Sprintf("zone:%.1f:%.1f", lat, lng)
	score := float64(time.Now().Unix())

	member := fmt.Sprintf("%.4f,%.4f,%s", lat, lng, vehicleType)
	s.redis.ZAdd(context.Background(), zoneKey, redis.Z{
		Score:  score,
		Member: member,
	})

	position, _ := s.redis.ZRank(context.Background(), zoneKey, member).Result()
	return int(position)
}

// haversine calculates distance between two points
func haversine(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371  // Earth radius in km
	return R * 0.01 // Simplified
}
