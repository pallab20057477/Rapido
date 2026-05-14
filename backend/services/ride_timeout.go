package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"
	"rapido-backend/websocket"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RideTimeoutService handles automatic ride timeouts and cancellations
type RideTimeoutService struct {
	redis  *redis.Client
	ctx    context.Context
	cancel context.CancelFunc
}

// NewRideTimeoutService creates timeout service
func NewRideTimeoutService() *RideTimeoutService {
	ctx, cancel := context.WithCancel(context.Background())
	return &RideTimeoutService{
		redis:  database.RedisClient,
		ctx:    ctx,
		cancel: cancel,
	}
}

// ScheduleRideTimeout schedules auto-cancel for a ride request
func (s *RideTimeoutService) ScheduleRideTimeout(rideID uuid.UUID, timeoutMinutes int) {
	key := fmt.Sprintf("ride:timeout:%s", rideID.String())
	expireAt := time.Now().Add(time.Duration(timeoutMinutes) * time.Minute)

	// Store timeout info in Redis
	s.redis.Set(s.ctx, key, expireAt.Unix(), time.Duration(timeoutMinutes+1)*time.Minute)

	utils.Info("Ride timeout scheduled",
		zap.String("ride_id", rideID.String()),
		zap.Int("timeout_min", timeoutMinutes),
		zap.Time("expires_at", expireAt))
}

// ScheduleDriverAssignmentTimeout schedules timeout for driver assignment
func (s *RideTimeoutService) ScheduleDriverAssignmentTimeout(rideID uuid.UUID, driverID uuid.UUID, timeoutMinutes int) {
	key := fmt.Sprintf("ride:driver_timeout:%s", rideID.String())

	data := map[string]interface{}{
		"ride_id":   rideID.String(),
		"driver_id": driverID.String(),
		"expire_at": time.Now().Add(time.Duration(timeoutMinutes) * time.Minute).Unix(),
	}

	dataBytes, _ := json.Marshal(data)
	if s.redis != nil {
		s.redis.Set(s.ctx, key, dataBytes, time.Duration(timeoutMinutes+1)*time.Minute)
	}
}

// StartTimeoutMonitor starts background monitor for ride timeouts
func (s *RideTimeoutService) StartTimeoutMonitor() {
	utils.Info("Starting ride timeout monitor")

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				utils.Info("Ride timeout monitor stopping")
				return
			case <-ticker.C:
				s.processTimeouts()
			}
		}
	}()
}

// StopTimeoutMonitor signals the monitor to stop
func (s *RideTimeoutService) StopTimeoutMonitor() {
	if s.cancel != nil {
		s.cancel()
	}
}

// parseUnixFromString parses a unix timestamp from string input which may be an int or float string
func parseUnixFromString(val string) (int64, error) {
	v := strings.TrimSpace(val)
	if v == "" {
		return 0, fmt.Errorf("empty value")
	}

	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i, nil
	}

	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return int64(f), nil
	}

	return 0, fmt.Errorf("unable to parse unix time: %s", v)
}

// processTimeouts checks and processes expired timeouts
func (s *RideTimeoutService) processTimeouts() {
	now := time.Now().Unix()

	// 1. Check ride request timeouts (no driver found)
	s.processRideRequestTimeouts(now)

	// 2. Check driver assignment timeouts (driver accepted but didn't move)
	s.processDriverAssignmentTimeouts(now)

	// 3. Check driver arrival timeouts
	s.processDriverArrivalTimeouts(now)

	// 4. Check ride start timeouts
	s.processRideStartTimeouts(now)
}

// processRideRequestTimeouts cancels rides with no driver
func (s *RideTimeoutService) processRideRequestTimeouts(now int64) {
	// Find all ride timeout keys
	if s.redis == nil {
		return
	}

	keys, err := s.redis.Keys(s.ctx, "ride:timeout:*").Result()
	if err != nil {
		return
	}

	for _, key := range keys {
		val, err := s.redis.Get(s.ctx, key).Result()
		if err != nil {
			continue
		}

		// value stored as unix timestamp string or int64; try parsing safely
		var expireUnix int64
		if expireUnix, err = parseUnixFromString(val); err != nil {
			continue
		}

		if now >= expireUnix {
			// Extract ride ID from key prefix
			rideIDStr := strings.TrimPrefix(key, "ride:timeout:")
			rideIDStr = strings.TrimSpace(rideIDStr)
			rideID, err := uuid.Parse(rideIDStr)
			if err != nil {
				continue
			}

			s.cancelRideNoDriver(rideID)

			// Clean up
			s.redis.Del(s.ctx, key)
		}
	}
}

// cancelRideNoDriver cancels ride when no driver found
func (s *RideTimeoutService) cancelRideNoDriver(rideID uuid.UUID) {
	var ride models.Ride
	if err := database.DB.First(&ride, rideID).Error; err != nil {
		return
	}

	// Only cancel if still in requested state
	if ride.Status != models.RideStatusRequested {
		return
	}

	now := time.Now()

	// Update ride status
	database.DB.Model(&ride).Updates(map[string]interface{}{
		"status":       models.RideStatusNoDriverFound,
		"cancelled_at": now,
		"updated_at":   now,
	})

	// Notify rider
	websocket.GetHandler().SendToUserEvent(ride.RiderID.String(), "ride_status_update", map[string]interface{}{
		"ride_id": rideID.String(),
		"status":  models.RideStatusNoDriverFound,
		"reason":  "no_driver_available",
		"message": "No drivers available in your area. Please try again.",
	})

	// Remove from surge tracking
	surgeService := NewSurgePricingService()
	surgeService.RecordRideCompletion(rideID.String(), ride.Pickup.Latitude, ride.Pickup.Longitude, ride.VehicleType)

	// Clear caches
	database.DeleteCache(database.GetRiderCurrentRideKey(ride.RiderID.String()))

	utils.Info("Ride auto-cancelled: no driver found",
		zap.String("ride_id", rideID.String()),
		zap.String("rider_id", ride.RiderID.String()))
}

// processDriverAssignmentTimeouts handles driver idle after acceptance
func (s *RideTimeoutService) processDriverAssignmentTimeouts(now int64) {
	if s.redis == nil {
		return
	}

	keys, err := s.redis.Keys(s.ctx, "ride:driver_timeout:*").Result()
	if err != nil {
		return
	}

	for _, key := range keys {
		data, err := s.redis.Get(s.ctx, key).Result()
		if err != nil {
			continue
		}

		var timeoutData map[string]interface{}
		if err := utils.FromJSON(data, &timeoutData); err != nil {
			continue
		}

		// Safely extract expire_at
		var expireAt int64
		if raw, ok := timeoutData["expire_at"]; ok {
			switch v := raw.(type) {
			case float64:
				expireAt = int64(v)
			case int64:
				expireAt = v
			case string:
				if parsed, err := parseUnixFromString(v); err == nil {
					expireAt = parsed
				} else {
					continue
				}
			default:
				continue
			}
		} else {
			continue
		}

		if now >= expireAt {
			rideIDStr, _ := timeoutData["ride_id"].(string)
			driverIDStr, _ := timeoutData["driver_id"].(string)

			rideID, err := uuid.Parse(strings.TrimSpace(rideIDStr))
			if err != nil {
				continue
			}
			driverID, err := uuid.Parse(strings.TrimSpace(driverIDStr))
			if err != nil {
				continue
			}

			s.handleDriverIdleTimeout(rideID, driverID)
			s.redis.Del(s.ctx, key)
		}
	}
}

// handleDriverIdleTimeout handles driver who accepted but didn't move
func (s *RideTimeoutService) handleDriverIdleTimeout(rideID, driverID uuid.UUID) {
	var ride models.Ride
	if err := database.DB.First(&ride, rideID).Error; err != nil {
		return
	}

	// Only process if still assigned to this driver
	if ride.Status != models.RideStatusDriverAssigned || ride.DriverID.String() != driverID.String() {
		return
	}

	// Release driver lock
	LockMgr.ForceUnlock(rideID.String())
	LockMgr.ForceUnlock(driverID.String())

	// Reopen ride for other drivers
	now := time.Now()
	database.DB.Model(&ride).Updates(map[string]interface{}{
		"driver_id":   nil,
		"status":      models.RideStatusRequested,
		"accepted_at": nil,
		"updated_at":  now,
	})

	// Penalize driver
	database.DB.Exec("UPDATE drivers SET reliability_score = reliability_score - 5 WHERE id = ?", driverID)

	// Notify driver
	websocket.GetHandler().SendToUserEvent(driverID.String(), "ride_timeout", map[string]interface{}{
		"ride_id": rideID.String(),
		"reason":  "idle_timeout",
		"message": "Ride reassigned due to inactivity",
	})

	// Re-notify other drivers
	go NewMatchingService().StartMatchingProcess(&ride)

	utils.Warn("Driver idle timeout - ride reassigned",
		zap.String("ride_id", rideID.String()),
		zap.String("driver_id", driverID.String()))
}

// processDriverArrivalTimeouts handles drivers who never arrive
func (s *RideTimeoutService) processDriverArrivalTimeouts(now int64) {
	// Find rides where driver is assigned but hasn't arrived for 15+ minutes
	var rides []models.Ride
	database.DB.Where("status = ? AND accepted_at < ?",
		models.RideStatusDriverAssigned,
		time.Now().Add(-15*time.Minute)).Find(&rides)

	for _, ride := range rides {
		s.handleDriverArrivalTimeout(ride.ID, *ride.DriverID)
	}
}

// handleDriverArrivalTimeout handles driver who never arrived
func (s *RideTimeoutService) handleDriverArrivalTimeout(rideID, driverID uuid.UUID) {
	// Similar to idle timeout but with stronger penalty
	s.handleDriverIdleTimeout(rideID, driverID)

	// Stronger penalty for no-show
	database.DB.Model(&models.Driver{}).Where("id = ?", driverID).
		Updates(map[string]interface{}{
			"is_online":         false,
			"reliability_score": gorm.Expr("reliability_score - 20"),
		})

	utils.Error("Driver no-show penalty applied",
		zap.String("ride_id", rideID.String()),
		zap.String("driver_id", driverID.String()))
}

// processRideStartTimeouts handles rides that never started
func (s *RideTimeoutService) processRideStartTimeouts(now int64) {
	// Find rides stuck in "arrived" state for too long
	var rides []models.Ride
	database.DB.Where("status = ? AND driver_arrived_at < ?",
		models.RideStatusDriverArrived,
		time.Now().Add(-10*time.Minute)).Find(&rides)

	for _, ride := range rides {
		s.handleRideStartTimeout(ride.ID, ride.RiderID, *ride.DriverID)
	}
}

// handleRideStartTimeout handles ride that never started after driver arrived
func (s *RideTimeoutService) handleRideStartTimeout(rideID, riderID, driverID uuid.UUID) {
	// Cancel the ride
	now := time.Now()
	database.DB.Model(&models.Ride{}).Where("id = ?", rideID).Updates(map[string]interface{}{
		"status":        models.RideStatusCancelled,
		"cancelled_at":  now,
		"cancelled_by":  "system",
		"cancel_reason": "rider_no_show",
	})

	// Release locks
	LockMgr.ForceUnlock(rideID.String())
	LockMgr.ForceUnlock(driverID.String())

	// Notify both parties
	websocket.GetHandler().SendToUserEvent(riderID.String(), "ride_cancelled", map[string]interface{}{
		"ride_id": rideID.String(),
		"reason":  "timeout",
	})

	websocket.GetHandler().SendToUserEvent(driverID.String(), "ride_cancelled", map[string]interface{}{
		"ride_id": rideID.String(),
		"reason":  "rider_no_show",
	})

	utils.Info("Ride cancelled: rider no-show after driver arrival",
		zap.String("ride_id", rideID.String()))
}

// CancelScheduledTimeout cancels a scheduled timeout
func (s *RideTimeoutService) CancelScheduledTimeout(rideID uuid.UUID) {
	key := fmt.Sprintf("ride:timeout:%s", rideID.String())
	s.redis.Del(s.ctx, key)
}

// Global instance
var TimeoutService *RideTimeoutService

// InitTimeoutService initializes the timeout service
func InitTimeoutService() {
	TimeoutService = NewRideTimeoutService()
	TimeoutService.StartTimeoutMonitor()
	utils.Info("Ride timeout service initialized")
}
