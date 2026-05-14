package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// FraudDetectionService detects and prevents fraudulent activity
type FraudDetectionService struct {
	redis *redis.Client
	ctx   context.Context
}

// FraudScore represents risk assessment
type FraudScore struct {
	Score       float64  // 0-100, higher = more risky
	RiskLevel   string   // low, medium, high, critical
	Flags       []string // List of triggered flags
	Action      string   // allow, review, block
}

// NewFraudDetectionService creates fraud detection service
func NewFraudDetectionService() *FraudDetectionService {
	return &FraudDetectionService{
		redis: database.RedisClient,
		ctx:   context.Background(),
	}
}

// CheckRideRequest fraud checks for new ride
func (f *FraudDetectionService) CheckRideRequest(userID string, pickupLat, pickupLng float64) *FraudScore {
	flags := []string{}
	score := 0.0
	
	// Check 1: Rapid ride requests (spam)
	if f.isRapidRequesting(userID) {
		score += 30
		flags = append(flags, "rapid_requests")
	}
	
	// Check 2: GPS spoofing detection
	if f.isGPSSuspicious(userID, pickupLat, pickupLng) {
		score += 50
		flags = append(flags, "gps_anomaly")
	}
	
	// Check 3: Unusual location
	if f.isUnusualLocation(userID, pickupLat, pickupLng) {
		score += 20
		flags = append(flags, "unusual_location")
	}
	
	// Check 4: New account abuse
	if f.isNewAccountHighRisk(userID) {
		score += 25
		flags = append(flags, "new_account_high_activity")
	}
	
	return f.calculateRisk(score, flags)
}

// CheckDriverFraud fraud checks for driver
func (f *FraudDetectionService) CheckDriverFraud(driverID string, lat, lng float64) *FraudScore {
	flags := []string{}
	score := 0.0
	
	// Check 1: GPS spoofing (teleportation detection)
	if f.isDriverTeleporting(driverID, lat, lng) {
		score += 60
		flags = append(flags, "driver_gps_spoofing")
	}
	
	// Check 2: Ride looping (fake rides)
	if f.isRideLooping(driverID) {
		score += 70
		flags = append(flags, "ride_looping_detected")
	}
	
	// Check 3: Abnormal acceptance pattern
	if f.isAbnormalAcceptance(driverID) {
		score += 40
		flags = append(flags, "abnormal_acceptance")
	}
	
	return f.calculateRisk(score, flags)
}

// CheckPaymentFraud fraud checks for payment
func (f *FraudDetectionService) CheckPaymentFraud(userID string, amount float64, method string) *FraudScore {
	flags := []string{}
	score := 0.0
	
	// Check 1: Rapid payment attempts
	if f.isRapidPaymentAttempt(userID) {
		score += 35
		flags = append(flags, "rapid_payment_attempts")
	}
	
	// Check 2: Amount anomaly
	if f.isAmountAnomalous(userID, amount) {
		score += 25
		flags = append(flags, "amount_anomaly")
	}
	
	// Check 3: Payment method switching
	if f.isMethodSwitching(userID) {
		score += 20
		flags = append(flags, "method_switching")
	}
	
	return f.calculateRisk(score, flags)
}

// isRapidRequesting checks if user is requesting rides too fast
func (f *FraudDetectionService) isRapidRequesting(userID string) bool {
	key := fmt.Sprintf("fraud:ride_requests:%s", userID)
	
	// Count requests in last 5 minutes
	now := time.Now().Unix()
	window := now - 300 // 5 minutes
	
	// Add current request
	f.redis.ZAdd(f.ctx, key, redis.Z{Score: float64(now), Member: now})
	f.redis.Expire(f.ctx, key, 10*time.Minute)
	
	// Count in window
	count, _ := f.redis.ZCount(f.ctx, key, fmt.Sprintf("%d", window), fmt.Sprintf("%d", now)).Result()
	
	return count > 5 // More than 5 requests in 5 minutes
}

// isGPSSuspicious detects GPS spoofing/jumping
func (f *FraudDetectionService) isGPSSuspicious(userID string, lat, lng float64) bool {
	key := fmt.Sprintf("fraud:locations:%s", userID)
	
	// Get last location
	lastData, err := f.redis.Get(f.ctx, key).Result()
	if err != nil {
		// No previous location, store and return
		f.redis.Set(f.ctx, key, fmt.Sprintf("%f,%f,%d", lat, lng, time.Now().Unix()), 30*time.Minute)
		return false
	}
	
	var lastLat, lastLng float64
	var lastTime int64
	fmt.Sscanf(lastData, "%f,%f,%d", &lastLat, &lastLng, &lastTime)
	
	// Calculate distance and time diff
	distance := utils.CalculateDistance(lastLat, lastLng, lat, lng)
	timeDiff := time.Now().Unix() - lastTime
	
	// Update location
	f.redis.Set(f.ctx, key, fmt.Sprintf("%f,%f,%d", lat, lng, time.Now().Unix()), 30*time.Minute)
	
	// Check for impossible movement (faster than 100 km/h average)
	if timeDiff > 0 {
		speed := (distance / float64(timeDiff)) * 3600 // km/h
		if speed > 100 {
			return true
		}
	}
	
	return false
}

// isDriverTeleporting detects driver GPS spoofing
func (f *FraudDetectionService) isDriverTeleporting(driverID string, lat, lng float64) bool {
	key := fmt.Sprintf("fraud:driver_loc:%s", driverID)
	
	// Get recent locations (last 3)
	locations, _ := f.redis.LRange(f.ctx, key, 0, 2).Result()
	
	// Add new location
	locData := fmt.Sprintf("%f,%f,%d", lat, lng, time.Now().Unix())
	f.redis.LPush(f.ctx, key, locData)
	f.redis.LTrim(f.ctx, key, 0, 9) // Keep last 10
	f.redis.Expire(f.ctx, key, 1*time.Hour)
	
	if len(locations) < 2 {
		return false
	}
	
	// Parse last two locations
	var lat1, lng1 float64
	var t1 int64
	fmt.Sscanf(locations[0], "%f,%f,%d", &lat1, &lng1, &t1)
	
	var lat2, lng2 float64
	var t2 int64
	fmt.Sscanf(locations[1], "%f,%f,%d", &lat2, &lng2, &t2)
	
	// Check for teleportation (> 200 km/h for 2 consecutive updates)
	distance := utils.CalculateDistance(lat1, lng1, lat2, lng2)
	timeDiff := t1 - t2
	
	if timeDiff > 0 {
		speed := (distance / float64(timeDiff)) * 3600
		if speed > 200 { // Impossible speed
			return true
		}
	}
	
	return false
}

// isRideLooping detects fake ride patterns
func (f *FraudDetectionService) isRideLooping(driverID string) bool {
	// Check for repeated start/end locations
	key := fmt.Sprintf("fraud:ride_patterns:%s", driverID)
	
	count, _ := f.redis.Get(f.ctx, key).Int()
	if count > 5 { // Same pattern repeated >5 times
		return true
	}
	
	return false
}

// isAbnormalAcceptance checks for bot-like acceptance
func (f *FraudDetectionService) isAbnormalAcceptance(driverID string) bool {
	// Check if driver accepts every ride instantly
	key := fmt.Sprintf("fraud:accept_pattern:%s", driverID)
	
	acceptCount, _ := f.redis.Get(f.ctx, key+":accept").Int()
	rejectCount, _ := f.redis.Get(f.ctx, key+":reject").Int()
	
	total := acceptCount + rejectCount
	if total > 10 {
		acceptRate := float64(acceptCount) / float64(total)
		if acceptRate > 0.95 { // Accepts 95%+ of rides (unnatural)
			return true
		}
	}
	
	return false
}

// isUnusualLocation checks if pickup is far from user's normal area
func (f *FraudDetectionService) isUnusualLocation(userID string, lat, lng float64) bool {
	// Simplified: Check if > 50km from last pickup
	key := fmt.Sprintf("fraud:user_area:%s", userID)
	
	areaData, err := f.redis.Get(f.ctx, key).Result()
	if err != nil {
		// First ride, store location
		f.redis.Set(f.ctx, key, fmt.Sprintf("%f,%f", lat, lng), 30*24*time.Hour)
		return false
	}
	
	var usualLat, usualLng float64
	fmt.Sscanf(areaData, "%f,%f", &usualLat, &usualLng)
	
	distance := utils.CalculateDistance(usualLat, usualLng, lat, lng)
	return distance > 50 // More than 50km from usual area
}

// isNewAccountHighRisk checks if new account has suspicious activity
func (f *FraudDetectionService) isNewAccountHighRisk(userID string) bool {
	// Check account age vs activity level
	// Simplified implementation
	return false
}

// isRapidPaymentAttempt checks for payment spam
func (f *FraudDetectionService) isRapidPaymentAttempt(userID string) bool {
	key := fmt.Sprintf("fraud:payments:%s", userID)
	
	now := time.Now().Unix()
	window := now - 60 // 1 minute
	
	count, _ := f.redis.ZCount(f.ctx, key, 
		fmt.Sprintf("%d", window), 
		fmt.Sprintf("%d", now)).Result()
	
	// Record attempt
	f.redis.ZAdd(f.ctx, key, redis.Z{Score: float64(now), Member: now})
	f.redis.Expire(f.ctx, key, 10*time.Minute)
	
	return count > 3 // More than 3 payment attempts in 1 minute
}

// isAmountAnomalous checks for unusual payment amounts
func (f *FraudDetectionService) isAmountAnomalous(userID string, amount float64) bool {
	// Check if amount is significantly higher than user's average
	// Simplified: flag if > 1000 INR (unusual for bike/auto)
	return amount > 1000
}

// isMethodSwitching detects payment method hopping
func (f *FraudDetectionService) isMethodSwitching(userID string) bool {
	key := fmt.Sprintf("fraud:payment_methods:%s", userID)
	
	methods, _ := f.redis.SCard(f.ctx, key).Result()
	return methods > 3 // Used more than 3 different methods recently
}

// calculateRisk determines risk level and action
func (f *FraudDetectionService) calculateRisk(score float64, flags []string) *FraudScore {
	score = math.Min(100, score)
	
	var level, action string
	
	switch {
	case score < 30:
		level = "low"
		action = "allow"
	case score < 50:
		level = "medium"
		action = "allow" // But log for review
	case score < 70:
		level = "high"
		action = "review"
	default:
		level = "critical"
		action = "block"
	}
	
	return &FraudScore{
		Score:     score,
		RiskLevel: level,
		Flags:     flags,
		Action:    action,
	}
}

// LogFraudEvent logs fraud detection for analysis
func (f *FraudDetectionService) LogFraudEvent(userID, eventType string, score *FraudScore) {
	utils.Warn("Fraud detection triggered",
		zap.String("user_id", userID),
		zap.String("event_type", eventType),
		zap.Float64("score", score.Score),
		zap.String("risk_level", score.RiskLevel),
		zap.Strings("flags", score.Flags),
	)
	
	// Store in Redis for real-time monitoring
	key := fmt.Sprintf("fraud:events:%s", time.Now().Format("2006-01-02"))
	eventData := fmt.Sprintf("%s|%s|%s|%f|%s", 
		time.Now().Format("15:04:05"),
		userID, 
		eventType, 
		score.Score,
		score.RiskLevel)
	
	f.redis.LPush(f.ctx, key, eventData)
	f.redis.Expire(f.ctx, key, 7*24*time.Hour)
}

// Global instance
var FraudDetector *FraudDetectionService

// InitFraudDetection initializes fraud detection
func InitFraudDetection() {
	FraudDetector = NewFraudDetectionService()
	utils.Info("Fraud detection service initialized")
}
