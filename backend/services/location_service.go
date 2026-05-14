package services

import (
	"encoding/json"
	"fmt"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"

	"github.com/google/uuid"
)

// LocationBatchService handles batched location writes to reduce DB load
type LocationBatchService struct {
	batchSize     int
	flushInterval time.Duration
	buffer        map[string][]LocationPoint // ride_id/driver_id -> locations
}

// LocationPoint represents a GPS coordinate with timestamp
type LocationPoint struct {
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Speed     float64   `json:"speed"`
	Heading   float64   `json:"heading"`
	Accuracy  float64   `json:"accuracy"`
	Timestamp time.Time `json:"timestamp"`
}

// NewLocationBatchService creates a new batch service
func NewLocationBatchService() *LocationBatchService {
	lbs := &LocationBatchService{
		batchSize:     10,               // Write to DB every 10 points
		flushInterval: 15 * time.Second, // Or every 15 seconds
		buffer:        make(map[string][]LocationPoint),
	}

	// Start background flusher
	go lbs.backgroundFlusher()

	return lbs
}

// StoreLocation stores location in Redis (real-time) and batches for DB
func (lbs *LocationBatchService) StoreLocation(entityID string, point LocationPoint, isDriver bool) error {
	// 1. Always update Redis for real-time tracking (expires in 5 min)
	redisKey := fmt.Sprintf("location:%s", entityID)
	locationJSON, _ := json.Marshal(point)
	if err := database.SetCache(redisKey, string(locationJSON), 5*time.Minute); err != nil {
		return err
	}

	// 2. Add to batch buffer
	lbs.buffer[entityID] = append(lbs.buffer[entityID], point)

	// 3. Flush if batch size reached
	if len(lbs.buffer[entityID]) >= lbs.batchSize {
		return lbs.flushToDB(entityID, isDriver)
	}

	return nil
}

// GetCurrentLocation gets real-time location from Redis
func (lbs *LocationBatchService) GetCurrentLocation(entityID string) (*LocationPoint, error) {
	redisKey := fmt.Sprintf("location:%s", entityID)
	data, err := database.GetCache(redisKey)
	if err != nil || data == "" {
		return nil, fmt.Errorf("location not found in cache")
	}

	var point LocationPoint
	if err := json.Unmarshal([]byte(data), &point); err != nil {
		return nil, err
	}

	return &point, nil
}

// flushToDB writes batched locations to database
func (lbs *LocationBatchService) flushToDB(entityID string, isDriver bool) error {
	points, exists := lbs.buffer[entityID]
	if !exists || len(points) == 0 {
		return nil
	}

	// Clear buffer
	delete(lbs.buffer, entityID)

	// Parse UUID
	entityUUID, err := uuid.Parse(entityID)
	if err != nil {
		return err
	}

	// Insert batch (only last point if driver location, all if ride tracking)
	if isDriver {
		// For drivers, only keep latest location
		lastPoint := points[len(points)-1]
		driverLocation := models.DriverLocation{
			DriverID:  entityUUID,
			Latitude:  lastPoint.Lat,
			Longitude: lastPoint.Lng,
			Accuracy:  lastPoint.Accuracy,
			Speed:     lastPoint.Speed,
			Heading:   lastPoint.Heading,
		}
		return database.DB.Save(&driverLocation).Error
	}

	// For rides, store path (but throttled)
	// Only save first and last point of batch to reduce writes
	if len(points) >= 2 {
		records := []models.RideLocation{
			{
				RideID:    entityUUID,
				Latitude:  points[0].Lat,
				Longitude: points[0].Lng,
				CreatedAt: points[0].Timestamp,
			},
			{
				RideID:    entityUUID,
				Latitude:  points[len(points)-1].Lat,
				Longitude: points[len(points)-1].Lng,
				CreatedAt: points[len(points)-1].Timestamp,
			},
		}
		return database.DB.Create(&records).Error
	}

	return nil
}

// backgroundFlusher periodically flushes all buffers
func (lbs *LocationBatchService) backgroundFlusher() {
	ticker := time.NewTicker(lbs.flushInterval)
	defer ticker.Stop()

	for range ticker.C {
		for entityID := range lbs.buffer {
			// Determine if driver or ride based on key format
			isDriver := false // You can add logic to determine this
			lbs.flushToDB(entityID, isDriver)
		}
	}
}

// FlushAll forces flush of all pending locations (call on shutdown)
func (lbs *LocationBatchService) FlushAll() {
	for entityID := range lbs.buffer {
		isDriver := false
		lbs.flushToDB(entityID, isDriver)
	}
}
