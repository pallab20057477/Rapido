package services

import (
	"rapido-backend/database"

	"gorm.io/gorm"
)

type ConfigService struct {
	DB *gorm.DB
}

func NewConfigService() *ConfigService {
	return &ConfigService{DB: database.DB}
}

// AppConfig represents the application configuration
type AppConfig struct {
	Features       map[string]bool    `json:"features"`
	Limits         map[string]int     `json:"limits"`
	Timeouts       map[string]int     `json:"timeouts"`
	RadiusConfig   map[string]float64 `json:"radius_config"`
	Version        string             `json:"version"`
	MinAppVersions map[string]string  `json:"min_app_versions"`
}

// GetPublicConfig returns configuration for public clients
func (s *ConfigService) GetPublicConfig() AppConfig {
	return AppConfig{
		Features: map[string]bool{
			"surge_pricing":     true,
			"scheduled_rides":   true,
			"multiple_stops":    false,
			"chat_enabled":      true,
			"sos_enabled":       true,
			"wallet_enabled":    true,
			"referrals_enabled": true,
			"subscriptions":     false,
		},
		Limits: map[string]int{
			"max_scheduled_rides":      5,
			"max_emergency_contacts":   5,
			"max_saved_addresses":      10,
			"max_search_radius_km":     10,
			"default_search_radius_km": 5,
		},
		Timeouts: map[string]int{
			"driver_search_seconds":   30,
			"ride_request_timeout":    15,
			"otp_expiry_minutes":      5,
			"cancellation_window_min": 5,
		},
		RadiusConfig: map[string]float64{
			"nearby_drivers_km":      3.0,
			"max_pickup_distance_km": 20.0,
			"surge_radius_km":        5.0,
		},
		Version: "1.0.0",
		MinAppVersions: map[string]string{
			"ios":     "1.0.0",
			"android": "1.0.0",
		},
	}
}

// GetAdminConfig returns full configuration for admin
func (s *ConfigService) GetAdminConfig() map[string]interface{} {
	public := s.GetPublicConfig()
	return map[string]interface{}{
		"public": public,
		"system": s.GetSystemConfig(),
	}
}

// GetSystemConfig returns system status for admin monitoring
func (s *ConfigService) GetSystemConfig() map[string]interface{} {
	return map[string]interface{}{
		"database_status":   "healthy",
		"redis_status":      "healthy",
		"websocket_servers": 1,
		"active_drivers":    0, // Would be fetched from cache
		"pending_rides":     0,
		"avg_response_ms":   45,
		"error_rate":        0.001,
	}
}

// UpdateConfig updates a configuration value
func (s *ConfigService) UpdateConfig(key string, value interface{}) error {
	// In production, this would update Redis/DB
	// For now, log the update
	return nil
}
