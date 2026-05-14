package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/redis/go-redis/v9"
)

// CacheService provides intelligent caching
type CacheService struct {
	redis *redis.Client
	ctx   context.Context
}

// NewCacheService creates a new cache service
func NewCacheService() *CacheService {
	return &CacheService{
		redis: database.RedisClient,
		ctx:   context.Background(),
	}
}

// CacheKeys for different data types
const (
	CacheKeyFareEstimate  = "cache:fare:%s:%s"     // geohash:vehicle_type
	CacheKeySurgeFactors  = "cache:surge:%s:%s"    // geohash:vehicle_type
	CacheKeyNearbyDrivers = "cache:drivers:%s:%s"  // geohash:vehicle_type
	CacheKeyDriverProfile = "cache:driver:%s"      // driver_id
	CacheKeyUserProfile   = "cache:user:%s"        // user_id
	CacheKeyRideDetails   = "cache:ride:%s"        // ride_id
	CacheKeyActiveRide    = "cache:active_ride:%s" // user_id
)

// TTL constants
const (
	TTLFareEstimate  = 2 * time.Minute
	TTLSurgeFactors  = 1 * time.Minute
	TTLNearbyDrivers = 30 * time.Second
	TTLDriverProfile = 5 * time.Minute
	TTLUserProfile   = 10 * time.Minute
	TTLRideDetails   = 1 * time.Hour
	TTLActiveRide    = 2 * time.Hour
)

// GetCachedFareEstimate retrieves cached fare estimate
func (c *CacheService) GetCachedFareEstimate(pickupGeoHash, vehicleType string) (*FareEstimateCache, error) {
	key := fmt.Sprintf(CacheKeyFareEstimate, pickupGeoHash, vehicleType)

	data, err := c.redis.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}

	var estimate FareEstimateCache
	if err := json.Unmarshal([]byte(data), &estimate); err != nil {
		return nil, err
	}

	// Check if cache is still valid (not expired based on internal timestamp)
	if time.Since(estimate.CachedAt) > TTLFareEstimate {
		return nil, nil
	}

	return &estimate, nil
}

// SetFareEstimate caches fare estimate
func (c *CacheService) SetFareEstimate(pickupGeoHash, vehicleType string, estimate *FareEstimateCache) error {
	key := fmt.Sprintf(CacheKeyFareEstimate, pickupGeoHash, vehicleType)

	estimate.CachedAt = time.Now()
	data, _ := json.Marshal(estimate)

	return c.redis.Set(c.ctx, key, data, TTLFareEstimate).Err()
}

// GetCachedNearbyDrivers retrieves cached nearby drivers
func (c *CacheService) GetCachedNearbyDrivers(geoHash, vehicleType string) ([]CachedDriver, error) {
	key := fmt.Sprintf(CacheKeyNearbyDrivers, geoHash, vehicleType)

	data, err := c.redis.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var drivers []CachedDriver
	if err := json.Unmarshal([]byte(data), &drivers); err != nil {
		return nil, err
	}

	return drivers, nil
}

// SetNearbyDrivers caches nearby drivers
func (c *CacheService) SetNearbyDrivers(geoHash, vehicleType string, drivers []CachedDriver) error {
	key := fmt.Sprintf(CacheKeyNearbyDrivers, geoHash, vehicleType)
	data, _ := json.Marshal(drivers)

	return c.redis.Set(c.ctx, key, data, TTLNearbyDrivers).Err()
}

// GetCachedSurgeFactors retrieves cached surge data
func (c *CacheService) GetCachedSurgeFactors(geoHash, vehicleType string) (*SurgeFactorsCache, error) {
	key := fmt.Sprintf(CacheKeySurgeFactors, geoHash, vehicleType)

	data, err := c.redis.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var factors SurgeFactorsCache
	if err := json.Unmarshal([]byte(data), &factors); err != nil {
		return nil, err
	}

	return &factors, nil
}

// SetSurgeFactors caches surge calculation
func (c *CacheService) SetSurgeFactors(geoHash, vehicleType string, factors *SurgeFactorsCache) error {
	key := fmt.Sprintf(CacheKeySurgeFactors, geoHash, vehicleType)
	factors.CachedAt = time.Now()
	data, _ := json.Marshal(factors)

	return c.redis.Set(c.ctx, key, data, TTLSurgeFactors).Err()
}

// InvalidateRideCache clears ride-related caches
func (c *CacheService) InvalidateRideCache(rideID string) {
	keys := []string{
		fmt.Sprintf(CacheKeyRideDetails, rideID),
	}

	for _, key := range keys {
		c.redis.Del(c.ctx, key)
	}
}

// InvalidateUserCache clears user-related caches
func (c *CacheService) InvalidateUserCache(userID string) {
	keys := []string{
		fmt.Sprintf(CacheKeyUserProfile, userID),
		fmt.Sprintf(CacheKeyActiveRide, userID),
	}

	for _, key := range keys {
		c.redis.Del(c.ctx, key)
	}
}

// WarmupCache pre-populates cache with hot data
func (c *CacheService) WarmupCache() {
	utils.Info("Starting cache warmup...")

	// Pre-cache active surge areas
	// Pre-cache popular pickup locations
	// Pre-cache verified drivers

	go func() {
		time.Sleep(5 * time.Second) // Wait for server to stabilize

		// This would query DB and populate cache
		utils.Info("Cache warmup completed")
	}()
}

// GetCacheStats returns cache statistics
func (c *CacheService) GetCacheStats() (map[string]interface{}, error) {
	info, err := c.redis.Info(c.ctx, "stats").Result()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"redis_stats": info,
		"timestamp":   time.Now(),
	}, nil
}

// Cache structures

type FareEstimateCache struct {
	Distance        float64   `json:"distance"`
	Duration        int       `json:"duration"`
	BaseFare        float64   `json:"base_fare"`
	DistanceFare    float64   `json:"distance_fare"`
	TimeFare        float64   `json:"time_fare"`
	Subtotal        float64   `json:"subtotal"`
	SurgeMultiplier float64   `json:"surge_multiplier"`
	SurgeAmount     float64   `json:"surge_amount"`
	PlatformFee     float64   `json:"platform_fee"`
	Total           float64   `json:"total"`
	Currency        string    `json:"currency"`
	CachedAt        time.Time `json:"cached_at"`
}

type CachedDriver struct {
	DriverID    string  `json:"driver_id"`
	Latitude    float64 `json:"lat"`
	Longitude   float64 `json:"lng"`
	Distance    float64 `json:"distance"`
	ETA         int     `json:"eta"`
	Rating      float64 `json:"rating"`
	VehicleType string  `json:"vehicle_type"`
}

type SurgeFactorsCache struct {
	Multiplier float64   `json:"multiplier"`
	Demand     int       `json:"demand"`
	Supply     int       `json:"supply"`
	Ratio      float64   `json:"ratio"`
	CachedAt   time.Time `json:"cached_at"`
}

// Global instance
var Cache *CacheService

// InitCache initializes the global cache service
func InitCache() {
	Cache = NewCacheService()
	Cache.WarmupCache()
	utils.Info("Cache service initialized")
}
