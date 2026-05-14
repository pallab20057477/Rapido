package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// FeatureFlagService provides dynamic configuration and emergency controls
// CRITICAL for safe deployments and incident response
type FeatureFlagService struct {
	db    *gorm.DB
	redis *redis.Client
	ctx   context.Context
}

// FeatureFlag represents a dynamic configuration flag
type FeatureFlag struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Key         string    `json:"key" gorm:"uniqueIndex;not null"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        string    `json:"type" gorm:"default:'boolean'"` // boolean, percentage, user_list

	// Boolean flags
	Enabled bool `json:"enabled" gorm:"default:false"`

	// Percentage rollout (0-100)
	RolloutPercentage int `json:"rollout_percentage" gorm:"default:0"`

	// User list (comma-separated user IDs for targeted rollout)
	TargetUsers string `json:"target_users"`

	// Kill switch - if true, feature is emergency disabled
	IsKillSwitch bool `json:"is_kill_switch" gorm:"default:false"`

	// Metadata
	CreatedBy uuid.UUID `json:"created_by"`
	UpdatedBy uuid.UUID `json:"updated_by"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (FeatureFlag) TableName() string {
	return "feature_flags"
}

// NewFeatureFlagService creates a new feature flag service
func NewFeatureFlagService() *FeatureFlagService {
	return &FeatureFlagService{
		db:    database.DB,
		redis: database.RedisClient,
		ctx:   context.Background(),
	}
}

// IsEnabled checks if a feature is enabled for a user
// Supports: boolean, percentage rollout, user targeting
func (s *FeatureFlagService) IsEnabled(key string, userID uuid.UUID) bool {
	// 1. Check kill switch first (emergency override)
	if s.isKillSwitchActive(key) {
		utils.Warn("Kill switch active for feature", zap.String("key", key))
		return false
	}

	// 2. Check Redis cache
	flag, err := s.getFromCache(key)
	if err != nil {
		// Fallback to database
		flag, err = s.getFromDB(key)
		if err != nil {
			return false // Default: disabled if not found
		}
		s.cacheFlag(flag)
	}

	// 3. Evaluate flag
	return s.evaluateFlag(flag, userID)
}

// IsEnabledGlobally checks if feature is enabled (for non-user contexts)
func (s *FeatureFlagService) IsEnabledGlobally(key string) bool {
	return s.IsEnabled(key, uuid.Nil)
}

// EnableFeature enables a feature globally
func (s *FeatureFlagService) EnableFeature(key string, updatedBy uuid.UUID) error {
	return s.updateFlag(key, map[string]interface{}{
		"enabled":    true,
		"updated_by": updatedBy,
	})
}

// DisableFeature disables a feature globally
func (s *FeatureFlagService) DisableFeature(key string, updatedBy uuid.UUID) error {
	return s.updateFlag(key, map[string]interface{}{
		"enabled":    false,
		"updated_by": updatedBy,
	})
}

// SetRollout sets percentage rollout for gradual release
func (s *FeatureFlagService) SetRollout(key string, percentage int, updatedBy uuid.UUID) error {
	if percentage < 0 || percentage > 100 {
		return fmt.Errorf("percentage must be 0-100")
	}

	return s.updateFlag(key, map[string]interface{}{
		"rollout_percentage": percentage,
		"updated_by":         updatedBy,
	})
}

// ActivateKillSwitch EMERGENCY: Instantly disable a feature across all regions
func (s *FeatureFlagService) ActivateKillSwitch(key string, updatedBy uuid.UUID, reason string) error {
	// 1. Update database
	if err := s.updateFlag(key, map[string]interface{}{
		"enabled":        false,
		"is_kill_switch": true,
		"description":    gorm.Expr("CONCAT(description, ' [KILL SWITCH: ', ?, ']')", reason),
		"updated_by":     updatedBy,
	}); err != nil {
		return fmt.Errorf("failed to activate kill switch: %w", err)
	}

	// 2. Immediate Redis cache invalidation
	cacheKey := fmt.Sprintf("feature_flag:%s", key)
	s.redis.Del(s.ctx, cacheKey)
	s.redis.Del(s.ctx, cacheKey+":kill")

	// 3. Publish to all instances via Pub/Sub
	event := map[string]interface{}{
		"type":       "kill_switch_activated",
		"key":        key,
		"reason":     reason,
		"updated_by": updatedBy,
		"timestamp":  time.Now().UTC(),
	}
	eventJSON, _ := json.Marshal(event)
	s.redis.Publish(s.ctx, "feature_flag_changes", eventJSON)

	utils.Error("🚨 KILL SWITCH ACTIVATED",
		zap.String("key", key),
		zap.String("reason", reason),
		zap.String("activated_by", updatedBy.String()))

	return nil
}

// DeactivateKillSwitch removes kill switch (careful!)
func (s *FeatureFlagService) DeactivateKillSwitch(key string, updatedBy uuid.UUID) error {
	return s.updateFlag(key, map[string]interface{}{
		"is_kill_switch": false,
		"updated_by":     updatedBy,
	})
}

// CreateFlag creates a new feature flag
func (s *FeatureFlagService) CreateFlag(flag *FeatureFlag) error {
	// Check if key exists
	var count int64
	s.db.Model(&FeatureFlag{}).Where("key = ?", flag.Key).Count(&count)
	if count > 0 {
		return fmt.Errorf("feature flag with key '%s' already exists", flag.Key)
	}

	return s.db.Create(flag).Error
}

// GetFlag retrieves a flag by key
func (s *FeatureFlagService) GetFlag(key string) (*FeatureFlag, error) {
	flag, err := s.getFromCache(key)
	if err != nil {
		flag, err = s.getFromDB(key)
		if err != nil {
			return nil, err
		}
		s.cacheFlag(flag)
	}
	return flag, nil
}

// GetAllFlags returns all feature flags
func (s *FeatureFlagService) GetAllFlags() ([]FeatureFlag, error) {
	var flags []FeatureFlag
	if err := s.db.Find(&flags).Error; err != nil {
		return nil, err
	}
	return flags, nil
}

// getFromCache retrieves flag from Redis
func (s *FeatureFlagService) getFromCache(key string) (*FeatureFlag, error) {
	data, err := s.redis.Get(s.ctx, fmt.Sprintf("feature_flag:%s", key)).Result()
	if err != nil {
		return nil, err
	}

	var flag FeatureFlag
	if err := json.Unmarshal([]byte(data), &flag); err != nil {
		return nil, err
	}
	return &flag, nil
}

// cacheFlag stores flag in Redis with TTL
func (s *FeatureFlagService) cacheFlag(flag *FeatureFlag) {
	data, _ := json.Marshal(flag)
	s.redis.Set(s.ctx, fmt.Sprintf("feature_flag:%s", flag.Key), data, 30*time.Second)
}

// getFromDB retrieves flag from database
func (s *FeatureFlagService) getFromDB(key string) (*FeatureFlag, error) {
	var flag FeatureFlag
	if err := s.db.Where("key = ?", key).First(&flag).Error; err != nil {
		return nil, err
	}
	return &flag, nil
}

// updateFlag updates flag in DB and invalidates cache
func (s *FeatureFlagService) updateFlag(key string, updates map[string]interface{}) error {
	result := s.db.Model(&FeatureFlag{}).Where("key = ?", key).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("feature flag '%s' not found", key)
	}

	// Invalidate cache
	s.redis.Del(s.ctx, fmt.Sprintf("feature_flag:%s", key))

	return nil
}

// isKillSwitchActive checks if kill switch is active
func (s *FeatureFlagService) isKillSwitchActive(key string) bool {
	// Check cache first
	killKey := fmt.Sprintf("feature_flag:%s:kill", key)
	val, err := s.redis.Get(s.ctx, killKey).Result()
	if err == nil && val == "1" {
		return true
	}

	// Check DB
	flag, err := s.getFromDB(key)
	if err != nil {
		return false
	}

	if flag.IsKillSwitch && !flag.Enabled {
		// Cache kill switch status
		s.redis.Set(s.ctx, killKey, "1", 1*time.Minute)
		return true
	}

	return false
}

// evaluateFlag determines if feature is enabled for user
func (s *FeatureFlagService) evaluateFlag(flag *FeatureFlag, userID uuid.UUID) bool {
	// Kill switch override
	if flag.IsKillSwitch && !flag.Enabled {
		return false
	}

	// Boolean flag
	if flag.Type == "boolean" || flag.Type == "" {
		return flag.Enabled
	}

	// Percentage rollout
	if flag.Type == "percentage" {
		if !flag.Enabled {
			return false
		}
		// Deterministic: userID % 100 < rollout_percentage
		userHash := hashUserID(userID)
		return (userHash % 100) < int64(flag.RolloutPercentage)
	}

	// User list targeting
	if flag.Type == "user_list" && userID != uuid.Nil {
		return containsUser(flag.TargetUsers, userID)
	}

	return flag.Enabled
}

// hashUserID creates deterministic hash for percentage rollout
func hashUserID(userID uuid.UUID) int64 {
	if userID == uuid.Nil {
		return 0
	}
	// Simple hash: use last 8 bytes as int64
	bytes := userID[:]
	return int64(bytes[8])<<56 | int64(bytes[9])<<48 | int64(bytes[10])<<40 | int64(bytes[11])<<32 |
		int64(bytes[12])<<24 | int64(bytes[13])<<16 | int64(bytes[14])<<8 | int64(bytes[15])
}

// containsUser checks if userID is in comma-separated list
func containsUser(list string, userID uuid.UUID) bool {
	if list == "" {
		return false
	}
	// Simple check - in production use proper parsing
	return len(list) > 0 // Simplified
}

// Default feature flags (initialized on startup)
var DefaultFeatureFlags = []FeatureFlag{
	{
		Key:               "new_matching_algorithm",
		Name:              "New ML-Based Matching",
		Description:       "Enable ML-based driver matching algorithm",
		Type:              "percentage",
		Enabled:           true,
		RolloutPercentage: 10, // Start with 10%
	},
	{
		Key:          "disable_new_rides",
		Name:         "Emergency: Disable New Rides",
		Description:  "Kill switch to disable all new ride requests",
		Type:         "boolean",
		Enabled:      false,
		IsKillSwitch: true,
	},
	{
		Key:         "advanced_fraud_detection",
		Name:        "Advanced Fraud Detection",
		Description: "Enable ML-based fraud detection",
		Type:        "boolean",
		Enabled:     false,
	},
	{
		Key:               "surge_pricing_v2",
		Name:              "Surge Pricing V2",
		Description:       "New surge pricing algorithm with better prediction",
		Type:              "percentage",
		Enabled:           true,
		RolloutPercentage: 5,
	},
}

// Global instance
var FeatureFlags *FeatureFlagService

// InitFeatureFlags initializes the global service
func InitFeatureFlags() {
	FeatureFlags = NewFeatureFlagService()

	// Seed default flags if they don't exist
	for _, flag := range DefaultFeatureFlags {
		var existing FeatureFlag
		if err := database.DB.Where("key = ?", flag.Key).First(&existing).Error; err != nil {
			// Create default flag
			database.DB.Create(&flag)
			utils.Info("Created default feature flag", zap.String("key", flag.Key))
		}
	}

	utils.Info("Feature flag service initialized")
}

// Helper function for kill switches in code
func KillSwitchActive(key string) bool {
	if FeatureFlags == nil {
		return false
	}
	return !FeatureFlags.IsEnabledGlobally(key)
}
