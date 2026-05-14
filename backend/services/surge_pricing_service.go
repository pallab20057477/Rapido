package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// SurgePricingService calculates dynamic surge multipliers based on supply-demand
type SurgePricingService struct {
	redis *redis.Client
}

// SurgeFactors holds the calculated surge parameters
type SurgeFactors struct {
	Multiplier  float64   `json:"multiplier"`
	BaseFare    float64   `json:"base_fare"`
	PerKMRate   float64   `json:"per_km_rate"`
	PerMinRate  float64   `json:"per_min_rate"`
	DemandScore int       `json:"demand_score"`
	SupplyScore int       `json:"supply_score"`
	Ratio       float64   `json:"ratio"`
	LastUpdated time.Time `json:"last_updated"`
}

// AreaGeoData represents a geographic area for surge calculation
type AreaGeoData struct {
	GeoHash   string
	CenterLat float64
	CenterLng float64
	RadiusKM  float64
}

// NewSurgePricingService creates a new surge pricing service
func NewSurgePricingService() *SurgePricingService {
	return &SurgePricingService{
		redis: database.RedisClient,
	}
}

// CalculateSurgeForArea calculates surge multiplier for a specific area
func (s *SurgePricingService) CalculateSurgeForArea(lat, lng float64, vehicleType string) (*SurgeFactors, error) {
	ctx := context.Background()

	// Get geohash for the area (precision 6 = ~1.2km)
	geoHash := utils.EncodeGeoHash(lat, lng, 6)

	// Get demand (active ride requests in this area)
	demandKey := fmt.Sprintf("surge:demand:%s:%s", geoHash, vehicleType)
	demand, err := s.redis.ZCard(ctx, demandKey).Result()
	if err != nil {
		demand = 0
	}

	// Get supply (online drivers in this area)
	supplyKey := fmt.Sprintf("drivers:online:%s", vehicleType)
	supply, err := s.redis.GeoRadius(ctx, supplyKey, lng, lat, &redis.GeoRadiusQuery{
		Radius: 3, // 3km radius
		Unit:   "km",
	}).Result()
	if err != nil {
		supply = nil
	}

	supplyCount := len(supply)

	// Calculate demand-supply ratio
	ratio := s.calculateRatio(int(demand), supplyCount)

	// Determine multiplier based on ratio
	multiplier := s.calculateMultiplier(ratio, int(demand))

	// Get base fare config
	baseFare, perKM, perMin := s.getFareConfig(vehicleType, multiplier)

	factors := &SurgeFactors{
		Multiplier:  multiplier,
		BaseFare:    baseFare,
		PerKMRate:   perKM,
		PerMinRate:  perMin,
		DemandScore: int(demand),
		SupplyScore: supplyCount,
		Ratio:       ratio,
		LastUpdated: time.Now(),
	}

	// Cache the result
	cacheKey := fmt.Sprintf("surge:active:%s:%s", geoHash, vehicleType)
	s.redis.Set(ctx, cacheKey, utils.MustJSON(factors), 5*time.Minute)

	utils.Info("Surge calculated",
		zap.String("geohash", geoHash),
		zap.String("vehicle_type", vehicleType),
		zap.Float64("multiplier", multiplier),
		zap.Int("demand", int(demand)),
		zap.Int("supply", supplyCount),
	)

	return factors, nil
}

// calculateRatio computes demand/supply ratio with safety checks
func (s *SurgePricingService) calculateRatio(demand, supply int) float64 {
	if supply == 0 {
		if demand > 0 {
			return 5.0 // Max surge if no drivers but demand exists
		}
		return 0.5 // Low activity
	}

	ratio := float64(demand) / float64(supply)

	// Cap the ratio
	if ratio > 5.0 {
		return 5.0
	}
	if ratio < 0.1 {
		return 0.1
	}

	return ratio
}

// calculateMultiplier determines surge multiplier based on ratio and demand
func (s *SurgePricingService) calculateMultiplier(ratio float64, demand int) float64 {
	var multiplier float64

	// Base multiplier from ratio
	switch {
	case ratio >= 4.0:
		multiplier = 2.5
	case ratio >= 3.0:
		multiplier = 2.0
	case ratio >= 2.0:
		multiplier = 1.5
	case ratio >= 1.5:
		multiplier = 1.3
	case ratio >= 1.0:
		multiplier = 1.2
	default:
		multiplier = 1.0
	}

	// Boost for high demand areas even with decent supply
	if demand >= 10 && multiplier < 1.5 {
		multiplier = math.Max(multiplier, 1.2)
	}

	// Cap at 3x (regulatory/UX limit)
	if multiplier > 3.0 {
		multiplier = 3.0
	}

	return multiplier
}

// getFareConfig returns base fare rates adjusted for surge
func (s *SurgePricingService) getFareConfig(vehicleType string, multiplier float64) (baseFare, perKM, perMin float64) {
	// Get from database or use defaults
	var fareConfig models.FareConfig
	database.DB.Where("vehicle_type = ?", vehicleType).First(&fareConfig)

	if fareConfig.ID == uuid.Nil {
		// Default configs
		switch vehicleType {
		case "bike":
			baseFare, perKM, perMin = 30, 10, 1.5
		case "auto":
			baseFare, perKM, perMin = 40, 15, 2.0
		case "cab":
			baseFare, perKM, perMin = 60, 20, 3.0
		default:
			baseFare, perKM, perMin = 30, 10, 1.5
		}
	} else {
		baseFare = fareConfig.BaseFare
		perKM = fareConfig.PerKmRate
		perMin = fareConfig.PerMinRate
	}

	// Apply surge to per km rate only (base fare stays same for transparency)
	return baseFare, perKM * multiplier, perMin * multiplier
}

// RecordRideRequest records a new ride request for surge calculation
func (s *SurgePricingService) RecordRideRequest(rideID string, lat, lng float64, vehicleType string) error {
	ctx := context.Background()
	geoHash := utils.EncodeGeoHash(lat, lng, 6)

	key := fmt.Sprintf("surge:demand:%s:%s", geoHash, vehicleType)

	// Add to sorted set with timestamp as score
	timestamp := float64(time.Now().Unix())
	err := s.redis.ZAdd(ctx, key, redis.Z{
		Score:  timestamp,
		Member: rideID,
	}).Err()

	// Set expiry on the key
	s.redis.Expire(ctx, key, 30*time.Minute)

	return err
}

// RecordRideCompletion removes ride from demand tracking
func (s *SurgePricingService) RecordRideCompletion(rideID string, lat, lng float64, vehicleType string) error {
	ctx := context.Background()
	geoHash := utils.EncodeGeoHash(lat, lng, 6)

	key := fmt.Sprintf("surge:demand:%s:%s", geoHash, vehicleType)

	return s.redis.ZRem(ctx, key, rideID).Err()
}

// CleanStaleDemand removes old ride requests from surge calculation
func (s *SurgePricingService) CleanStaleDemand() {
	ctx := context.Background()

	// Find all surge demand keys
	keys, err := s.redis.Keys(ctx, "surge:demand:*").Result()
	if err != nil {
		return
	}

	cutoff := float64(time.Now().Add(-30 * time.Minute).Unix())

	for _, key := range keys {
		// Remove entries older than 30 minutes
		s.redis.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%f", cutoff))

		// If empty, delete the key
		count, _ := s.redis.ZCard(ctx, key).Result()
		if count == 0 {
			s.redis.Del(ctx, key)
		}
	}
}

// GetSurgeForRideEstimate returns surge factors for fare estimation
func (s *SurgePricingService) GetSurgeForRideEstimate(pickupLat, pickupLng float64, vehicleType string) *SurgeFactors {
	// Try to get cached surge
	ctx := context.Background()
	geoHash := utils.EncodeGeoHash(pickupLat, pickupLng, 6)
	cacheKey := fmt.Sprintf("surge:active:%s:%s", geoHash, vehicleType)

	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var factors SurgeFactors
		if err := utils.FromJSON(cached, &factors); err == nil {
			return &factors
		}
	}

	// Calculate fresh
	factors, _ := s.CalculateSurgeForArea(pickupLat, pickupLng, vehicleType)
	return factors
}

// GetActiveSurgeAreas returns all areas with active surge
func (s *SurgePricingService) GetActiveSurgeAreas(vehicleType string) ([]SurgeAreaInfo, error) {
	ctx := context.Background()

	pattern := fmt.Sprintf("surge:active:*:%s", vehicleType)
	keys, err := s.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	var areas []SurgeAreaInfo

	for _, key := range keys {
		data, err := s.redis.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var factors SurgeFactors
		if err := utils.FromJSON(data, &factors); err != nil {
			continue
		}

		// Extract geohash from key
		var geoHash string
		fmt.Sscanf(key, "surge:active:%s", &geoHash)

		areas = append(areas, SurgeAreaInfo{
			GeoHash:    geoHash,
			Multiplier: factors.Multiplier,
			Demand:     factors.DemandScore,
			Supply:     factors.SupplyScore,
		})
	}

	return areas, nil
}

// SurgeAreaInfo represents a surge area for admin dashboard
type SurgeAreaInfo struct {
	GeoHash    string  `json:"geohash"`
	Multiplier float64 `json:"multiplier"`
	Demand     int     `json:"demand"`
	Supply     int     `json:"supply"`
}

// StartSurgeMonitoring starts background monitoring and recalculation
func (s *SurgePricingService) StartSurgeMonitoring() {
	ticker := time.NewTicker(2 * time.Minute)       // Recalculate every 2 minutes
	cleanTicker := time.NewTicker(10 * time.Minute) // Clean stale every 10 minutes

	go func() {
		for {
			select {
			case <-ticker.C:
				s.recalculateAllAreas()
			case <-cleanTicker.C:
				s.CleanStaleDemand()
			}
		}
	}()

	utils.Info("Surge pricing monitoring started")
}

// recalculateAllAreas recalculates surge for all active areas
func (s *SurgePricingService) recalculateAllAreas() {
	ctx := context.Background()

	// Find all areas with demand
	keys, err := s.redis.Keys(ctx, "surge:demand:*").Result()
	if err != nil {
		return
	}

	processed := make(map[string]bool)

	for _, key := range keys {
		// Extract geohash and vehicle type
		var geoHash, vehicleType string
		if _, err := fmt.Sscanf(key, "surge:demand:%s:%s", &geoHash, &vehicleType); err != nil {
			continue
		}

		// Decode geohash to lat/lng
		lat, lng := utils.DecodeGeoHash(geoHash)

		cacheKey := fmt.Sprintf("%s:%s", geoHash, vehicleType)
		if processed[cacheKey] {
			continue
		}
		processed[cacheKey] = true

		// Recalculate
		s.CalculateSurgeForArea(lat, lng, vehicleType)
	}
}
