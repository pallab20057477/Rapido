package services

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DriverLocationService handles high-frequency location updates with:
// - Throttling (max 1 update/sec per driver)
// - Batching (flush every 30s or 100 locations)
// - Redis primary storage for real-time queries
// - PostgreSQL async persistence for analytics
type DriverLocationService struct {
	redis    *redis.Client
	db       *gorm.DB
	buffer   *LocationBuffer
	eventBus *EventBus
	throttle *LocationThrottle
}

// LocationBuffer batches location updates for DB persistence
type LocationBuffer struct {
	mu            sync.Mutex
	locations     map[string]LocationUpdate
	lastFlush     time.Time
	maxSize       int
	flushInterval time.Duration
}

// LocationUpdate represents a driver location update
type LocationUpdate struct {
	DriverID  uuid.UUID
	Latitude  float64
	Longitude float64
	Accuracy  float64
	Speed     float64
	Heading   float64
	Timestamp time.Time
}

// LocationThrottle prevents excessive updates from a single driver
type LocationThrottle struct {
	mu          sync.RWMutex
	lastUpdates map[string]time.Time
	minInterval time.Duration
}

// NewDriverLocationService creates a location service with throttling and batching
func NewDriverLocationService() *DriverLocationService {
	service := &DriverLocationService{
		redis:    database.RedisClient,
		db:       database.DB,
		eventBus: EventBusInstance,
		buffer: &LocationBuffer{
			locations:     make(map[string]LocationUpdate),
			lastFlush:     time.Now(),
			maxSize:       100,
			flushInterval: 30 * time.Second,
		},
		throttle: &LocationThrottle{
			lastUpdates: make(map[string]time.Time),
			minInterval: 1 * time.Second, // Max 1 update per second
		},
	}

	// Start background flush goroutine
	go service.backgroundFlush()

	return service
}

// UpdateLocation handles driver location update with throttling
func (s *DriverLocationService) UpdateLocation(driverID uuid.UUID, lat, lng float64, accuracy, speed, heading float64, batteryLevel int) error {
	ctx := context.Background()

	// Throttle check - max 1 update per second per driver
	if !s.throttle.Allow(driverID.String()) {
		return nil // Silently drop throttled updates
	}

	now := time.Now()

	// Update Redis immediately (for real-time matching queries)
	locationKey := fmt.Sprintf("driver:%s:location", driverID.String())
	locationData := map[string]interface{}{
		"lat":       lat,
		"lng":       lng,
		"accuracy":  accuracy,
		"speed":     speed,
		"heading":   heading,
		"timestamp": now.Unix(),
		"battery":   batteryLevel,
	}

	// Store in Redis Hash with 5-minute TTL (check errors)
	if err := s.redis.HMSet(ctx, locationKey, locationData).Err(); err != nil {
		// Log and continue - non-fatal for update path
		utils.Warn("failed to HMSet driver location", zap.String("driver_id", driverID.String()), zap.String("err", err.Error()))
	}
	if err := s.redis.Expire(ctx, locationKey, 5*time.Minute).Err(); err != nil {
		utils.Warn("failed to set TTL for driver location", zap.String("driver_id", driverID.String()), zap.String("err", err.Error()))
	}

	// Update geo index for spatial queries (only if driver is online)
	onlineKey := database.GetDriverOnlineKey(driverID.String())
	if _, err := s.redis.Get(ctx, onlineKey).Result(); err == nil {
		// Driver is online, update centralized geo index via helper (checks/returns error)
		if err := database.UpdateDriverGeoLocation(driverID.String(), lat, lng); err != nil {
			utils.Warn("failed to update driver geo location", zap.String("driver_id", driverID.String()), zap.String("err", err.Error()))
		}
		// Ensure per-driver online key TTL is maintained by whoever marks driver online/offline.
		// Do NOT attempt to set TTL on a geo set member (not supported by Redis).
	}

	// Add to batch buffer for DB persistence
	update := LocationUpdate{
		DriverID:  driverID,
		Latitude:  lat,
		Longitude: lng,
		Accuracy:  accuracy,
		Speed:     speed,
		Heading:   heading,
		Timestamp: now,
	}
	s.buffer.Add(driverID.String(), update)

	// Emit location update event (for fraud detection, surge pricing)
	if s.eventBus != nil {
		go s.eventBus.PublishDriverLocationUpdated(driverID, lat, lng)
	}

	// Fraud detection check (every 10th update to save resources)
	if now.Unix()%10 == 0 {
		go s.detectAnomalies(driverID, lat, lng, speed)
	}

	return nil
}

// detectAnomalies checks for GPS spoofing and impossible movements
func (s *DriverLocationService) detectAnomalies(driverID uuid.UUID, lat, lng, speed float64) {
	ctx := context.Background()

	// Get last location from Redis
	locationKey := fmt.Sprintf("driver:%s:location", driverID.String())
	lastLocation, err := s.redis.HGetAll(ctx, locationKey).Result()
	if err != nil || len(lastLocation) == 0 {
		return
	}

	// Parse last location
	var lastLat, lastLng, lastTimestamp float64
	fmt.Sscanf(lastLocation["lat"], "%f", &lastLat)
	fmt.Sscanf(lastLocation["lng"], "%f", &lastLng)
	fmt.Sscanf(lastLocation["timestamp"], "%f", &lastTimestamp)

	// Calculate distance and time delta
	distance := calculateDistance(lastLat, lastLng, lat, lng)
	timeDelta := float64(time.Now().Unix()-int64(lastTimestamp)) / 3600 // hours

	if timeDelta > 0 {
		calculatedSpeed := distance / timeDelta // km/h

		// Check 1: Impossible speed (> 200 km/h)
		if calculatedSpeed > 200 {
			s.flagFraud(driverID, "impossible_speed", fmt.Sprintf("%.2f km/h", calculatedSpeed))
		}

		// Check 2: Reported speed vs calculated speed mismatch (> 50%)
		if speed > 0 && math.Abs(calculatedSpeed-speed) > 50 {
			s.flagFraud(driverID, "speed_mismatch", fmt.Sprintf("reported:%.2f, calculated:%.2f", speed, calculatedSpeed))
		}
	}
}

// flagFraud reports potential fraud to fraud detection service
func (s *DriverLocationService) flagFraud(driverID uuid.UUID, fraudType, details string) {
	// Create audit log entry for fraud
	alert := models.AuditLog{
		UserID:     &driverID,
		UserType:   "driver",
		Action:     "fraud_detected",
		EntityType: "driver",
		EntityID:   driverID.String(),
		NewValues:  models.JSONMap{"fraud_type": fraudType, "details": details},
		Status:     "failed",
		Severity:   "critical",
	}
	s.db.Create(&alert)

	// Publish fraud event
	if s.eventBus != nil {
		s.eventBus.Publish(EventFraudDetected, map[string]interface{}{
			"driver_id": driverID.String(),
			"type":      fraudType,
			"details":   details,
		}, nil)
	}
}

// GetDriverLocation retrieves driver's current location from Redis
func (s *DriverLocationService) GetDriverLocation(driverID uuid.UUID) (*LocationUpdate, error) {
	ctx := context.Background()
	locationKey := fmt.Sprintf("driver:%s:location", driverID.String())

	data, err := s.redis.HGetAll(ctx, locationKey).Result()
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no location found for driver")
	}

	var loc LocationUpdate
	loc.DriverID = driverID
	fmt.Sscanf(data["lat"], "%f", &loc.Latitude)
	fmt.Sscanf(data["lng"], "%f", &loc.Longitude)
	fmt.Sscanf(data["accuracy"], "%f", &loc.Accuracy)
	fmt.Sscanf(data["speed"], "%f", &loc.Speed)
	fmt.Sscanf(data["heading"], "%f", &loc.Heading)

	return &loc, nil
}

// FindNearbyDrivers queries drivers within radius using Redis GEO
func (s *DriverLocationService) FindNearbyDrivers(lat, lng, radiusKM float64, limit int) ([]redis.GeoLocation, error) {
	ctx := context.Background()

	results, err := s.redis.GeoRadius(ctx, "drivers:online", lng, lat, &redis.GeoRadiusQuery{
		Radius: radiusKM,
		Unit:   "km",
		Count:  limit,
	}).Result()

	if err != nil {
		return nil, err
	}

	return results, nil
}

// Allow checks if update is allowed based on throttling
func (t *LocationThrottle) Allow(driverID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	lastUpdate, exists := t.lastUpdates[driverID]

	if !exists || now.Sub(lastUpdate) >= t.minInterval {
		t.lastUpdates[driverID] = now
		return true
	}

	return false
}

// Add adds a location update to the buffer
func (b *LocationBuffer) Add(driverID string, update LocationUpdate) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.locations[driverID] = update
}

// backgroundFlush periodically flushes buffered locations to DB
func (s *DriverLocationService) backgroundFlush() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.flushBuffer()
	}
}

// flushBuffer writes buffered locations to PostgreSQL
func (s *DriverLocationService) flushBuffer() {
	s.buffer.mu.Lock()

	// Check if flush needed
	if len(s.buffer.locations) == 0 {
		s.buffer.mu.Unlock()
		return
	}

	if len(s.buffer.locations) < s.buffer.maxSize &&
		time.Since(s.buffer.lastFlush) < s.buffer.flushInterval {
		s.buffer.mu.Unlock()
		return
	}

	// Copy locations and clear buffer
	locations := make([]LocationUpdate, 0, len(s.buffer.locations))
	for _, loc := range s.buffer.locations {
		locations = append(locations, loc)
	}
	s.buffer.locations = make(map[string]LocationUpdate)
	s.buffer.lastFlush = time.Now()
	s.buffer.mu.Unlock()

	// Batch insert to database
	if len(locations) > 0 {
		s.batchInsertLocations(locations)
	}
}

// batchInsertLocations performs efficient batch insert
func (s *DriverLocationService) batchInsertLocations(locations []LocationUpdate) {
	// Use raw SQL for efficient batch insert
	sql := "INSERT INTO driver_locations (driver_id, latitude, longitude, accuracy, speed, heading, recorded_at) VALUES "
	vars := []interface{}{}

	for i, loc := range locations {
		if i > 0 {
			sql += ", "
		}
		sql += "(?, ?, ?, ?, ?, ?, ?)"
		vars = append(vars, loc.DriverID, loc.Latitude, loc.Longitude,
			loc.Accuracy, loc.Speed, loc.Heading, loc.Timestamp)
	}

	// Execute batch insert
	s.db.Exec(sql, vars...)

	// Clean old data (keep only last 24 hours)
	cutoff := time.Now().Add(-24 * time.Hour)
	s.db.Where("recorded_at < ?", cutoff).Delete(&models.DriverLocation{})
}

// GetLocationHistory retrieves driver's location history from DB
func (s *DriverLocationService) GetLocationHistory(driverID uuid.UUID, startTime, endTime time.Time) ([]models.DriverLocation, error) {
	var locations []models.DriverLocation

	err := s.db.Where("driver_id = ? AND recorded_at BETWEEN ? AND ?",
		driverID, startTime, endTime).
		Order("recorded_at DESC").
		Find(&locations).Error

	return locations, err
}

// Global instance
var DriverLocationServiceInstance *DriverLocationService

// InitDriverLocationService initializes the driver location service
func InitDriverLocationService() {
	DriverLocationServiceInstance = NewDriverLocationService()
}
