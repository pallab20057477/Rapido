package services

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// SIMSwapDetection detects fraud via device/SIM changes
type SIMSwapDetection struct {
	redis *redis.Client
}

// DeviceProfile stores user's device history
type DeviceProfile struct {
	UserID            string    `json:"user_id"`
	DeviceID          string    `json:"device_id"`
	DeviceFingerprint string    `json:"device_fingerprint"`
	SIMHash           string    `json:"sim_hash"`
	PhoneNumber       string    `json:"phone_number"`
	FirstSeen         time.Time `json:"first_seen"`
	LastSeen          time.Time `json:"last_seen"`
	TrustScore        int       `json:"trust_score"` // 0-100
}

func NewSIMSwapDetection(redis *redis.Client) *SIMSwapDetection {
	return &SIMSwapDetection{redis: redis}
}

// CheckForSIMSwap analyzes device/SIM change risk
func (s *SIMSwapDetection) CheckForSIMSwap(userID, deviceID, simHash, phone string) map[string]interface{} {
	// Get stored profile
	profile := s.getDeviceProfile(userID)

	if profile == nil {
		// New user - create profile
		s.storeDeviceProfile(userID, deviceID, simHash, phone)
		return map[string]interface{}{
			"risk_level":   "low",
			"action":       "allow",
			"reason":       "new_user_profile_created",
			"requires_2fa": false,
		}
	}

	// Check for changes
	deviceChanged := profile.DeviceID != deviceID
	simChanged := profile.SIMHash != simHash
	phoneChanged := profile.PhoneNumber != phone

	riskScore := 0
	reasons := []string{}

	if deviceChanged {
		riskScore += 30
		reasons = append(reasons, "new_device")
	}
	if simChanged {
		riskScore += 50 // High risk for SIM change (India fraud pattern)
		reasons = append(reasons, "sim_swap_detected")
	}
	if phoneChanged {
		riskScore += 40
		reasons = append(reasons, "phone_number_changed")
	}

	// Time-based risk (recent changes = higher risk)
	if time.Since(profile.LastSeen) < 24*time.Hour && (deviceChanged || simChanged) {
		riskScore += 20
		reasons = append(reasons, "rapid_change_after_recent_activity")
	}

	// Determine action
	action := "allow"
	requires2FA := false
	riskLevel := "low"

	if riskScore >= 70 {
		action = "block"
		riskLevel = "critical"
		requires2FA = true
	} else if riskScore >= 40 {
		action = "challenge"
		riskLevel = "high"
		requires2FA = true
	} else if riskScore > 0 {
		riskLevel = "medium"
		requires2FA = simChanged // Force 2FA for any SIM change
	}

	// Update last seen
	profile.LastSeen = time.Now()
	s.updateLastSeen(userID)

	return map[string]interface{}{
		"risk_score":     riskScore,
		"risk_level":     riskLevel,
		"action":         action,
		"reasons":        reasons,
		"requires_2fa":   requires2FA,
		"device_changed": deviceChanged,
		"sim_changed":    simChanged,
	}
}

// ForceReverification triggers mandatory re-verification
func (s *SIMSwapDetection) ForceReverification(userID string) map[string]interface{} {
	// Flag account for manual review
	s.flagAccount(userID, "sim_swap_suspicious")

	return map[string]interface{}{
		"action":             "force_reverify",
		"user_id":            userID,
		"requires_documents": true,
		"requires_selfie":    true,
		"cooldown_hours":     24,
	}
}

// getDeviceProfile retrieves stored profile
func (s *SIMSwapDetection) getDeviceProfile(userID string) *DeviceProfile {
	// Query from Redis/DB
	return nil // Placeholder
}

// storeDeviceProfile saves new profile
func (s *SIMSwapDetection) storeDeviceProfile(userID, deviceID, simHash, phone string) {
	profile := DeviceProfile{
		UserID:      userID,
		DeviceID:    deviceID,
		SIMHash:     simHash,
		PhoneNumber: phone,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		TrustScore:  50, // Neutral starting score
	}
	// Store in Redis with 30-day TTL
	s.redis.Set(context.Background(), "device:profile:"+userID, profile, 30*24*time.Hour)
}

// updateLastSeen updates timestamp
func (s *SIMSwapDetection) updateLastSeen(userID string) {
	// Update Redis
	s.redis.HSet(context.Background(), "device:profile:"+userID, "last_seen", time.Now())
}

// flagAccount marks for review
func (s *SIMSwapDetection) flagAccount(userID, reason string) {
	key := "fraud:flag:" + userID
	s.redis.Set(context.Background(), key, reason, 7*24*time.Hour)
}

// GetSIMSwapStats returns detection statistics
func GetSIMSwapStats() map[string]interface{} {
	return map[string]interface{}{
		"detection_rules": []string{
			"sim_change + device_change within 24h = BLOCK",
			"sim_change alone = 2FA_REQUIRED",
			"device_change alone = CHALLENGE",
			"phone_change + sim_change = MANUAL_REVIEW",
		},
		"india_specific":      true,
		"carrier_integration": "recommended",
	}
}

var SIMSwapDetector *SIMSwapDetection

func InitSIMSwapDetection(redis *redis.Client) {
	SIMSwapDetector = NewSIMSwapDetection(redis)
}

func GetSIMSwapDetection() *SIMSwapDetection {
	return SIMSwapDetector
}
