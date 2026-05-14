package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// IdempotencyService provides idempotency key storage and validation
type IdempotencyService struct {
	redis *redis.Client
	ctx   context.Context
}

// IdempotencyRecord stores request/response for deduplication
type IdempotencyRecord struct {
	Key          string          `json:"key"`
	RequestHash  string          `json:"request_hash"`
	Response     json.RawMessage `json:"response"`
	Status       string          `json:"status"` // pending, completed, failed
	CreatedAt    time.Time       `json:"created_at"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
}

// NewIdempotencyService creates a new idempotency service
func NewIdempotencyService() *IdempotencyService {
	return &IdempotencyService{
		redis: database.RedisClient,
		ctx:   context.Background(),
	}
}

// CheckAndStore checks if key exists, stores if new
func (s *IdempotencyService) CheckAndStore(key string, requestBody interface{}) (*IdempotencyRecord, bool, error) {
	// Generate request hash
	requestBytes, _ := json.Marshal(requestBody)
	requestHash := sha256.Sum256(requestBytes)
	hashStr := hex.EncodeToString(requestHash[:])
	
	// Check Redis first (fast path)
	existing, err := s.getFromRedis(key)
	if err == nil && existing != nil {
		// Key exists - check if request matches
		if existing.RequestHash == hashStr {
			// Same request - return cached response
			return existing, true, nil
		}
		// Different request with same key - conflict
		return nil, false, fmt.Errorf("idempotency key conflict: different request body")
	}
	
	// Check database (persistent storage)
	existing, err = s.getFromDB(key)
	if err == nil && existing != nil {
		if existing.RequestHash == hashStr {
			// Cache in Redis for future
			s.saveToRedis(existing, 24*time.Hour)
			return existing, true, nil
		}
		return nil, false, fmt.Errorf("idempotency key conflict: different request body")
	}
	
	// Create new record
	record := &IdempotencyRecord{
		Key:         key,
		RequestHash: hashStr,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
	
	// Store in both Redis and DB
	if err := s.saveToRedis(record, 24*time.Hour); err != nil {
		utils.Warn("Failed to cache idempotency record", zap.Error(err))
	}
	
	if err := s.saveToDB(record); err != nil {
		return nil, false, fmt.Errorf("failed to store idempotency record: %w", err)
	}
	
	return record, false, nil
}

// Complete marks an idempotency record as completed with response
func (s *IdempotencyService) Complete(key string, response interface{}) error {
	responseBytes, _ := json.Marshal(response)
	now := time.Now()
	
	// Update Redis
	record, _ := s.getFromRedis(key)
	if record != nil {
		record.Response = responseBytes
		record.Status = "completed"
		record.CompletedAt = &now
		s.saveToRedis(record, 24*time.Hour)
	}
	
	// Update database
	result := database.DB.Model(&IdempotencyRecord{}).
		Where("key = ?", key).
		Updates(map[string]interface{}{
			"response":     responseBytes,
			"status":       "completed",
			"completed_at": now,
		})
	
	if result.Error != nil {
		return result.Error
	}
	
	return nil
}

// Fail marks an idempotency record as failed
func (s *IdempotencyService) Fail(key string, errorMsg string) error {
	now := time.Now()
	
	result := database.DB.Model(&IdempotencyRecord{}).
		Where("key = ?", key).
		Updates(map[string]interface{}{
			"status":       "failed",
			"response":     json.RawMessage(`{"error":"` + errorMsg + `"}`),
			"completed_at": now,
		})
	
	return result.Error
}

// getFromRedis retrieves record from Redis
func (s *IdempotencyService) getFromRedis(key string) (*IdempotencyRecord, error) {
	data, err := s.redis.Get(s.ctx, "idempotency:"+key).Result()
	if err != nil {
		return nil, err
	}
	
	var record IdempotencyRecord
	if err := json.Unmarshal([]byte(data), &record); err != nil {
		return nil, err
	}
	
	return &record, nil
}

// saveToRedis stores record in Redis
func (s *IdempotencyService) saveToRedis(record *IdempotencyRecord, ttl time.Duration) error {
	data, _ := json.Marshal(record)
	return s.redis.Set(s.ctx, "idempotency:"+record.Key, data, ttl).Err()
}

// getFromDB retrieves record from database
func (s *IdempotencyService) getFromDB(key string) (*IdempotencyRecord, error) {
	var record IdempotencyRecord
	result := database.DB.Where("key = ?", key).First(&record)
	if result.Error != nil {
		return nil, result.Error
	}
	return &record, nil
}

// saveToDB stores record in database
func (s *IdempotencyService) saveToDB(record *IdempotencyRecord) error {
	return database.DB.Create(record).Error
}

// CleanupOldRecords removes completed records older than retention period
func (s *IdempotencyService) CleanupOldRecords(retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	
	result := database.DB.Where("status IN ? AND completed_at < ?", 
		[]string{"completed", "failed"}, cutoff).
		Delete(&IdempotencyRecord{})
	
	if result.Error != nil {
		return result.Error
	}
	
	utils.Info("Cleaned up old idempotency records", 
		zap.Int64("deleted", result.RowsAffected))
	
	return nil
}

// Global instance
var Idempotency *IdempotencyService

// InitIdempotency initializes the global service
func InitIdempotency() {
	Idempotency = NewIdempotencyService()
	utils.Info("Idempotency service initialized")
}
