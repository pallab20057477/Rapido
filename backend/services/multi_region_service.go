package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// MultiRegionService handles multi-region deployment and failover
type MultiRegionService struct {
	redis         *redis.Client
	currentRegion string
	regions       []Region
}

type Region struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Code            string   `json:"code"` // ap-south-1, ap-southeast-1
	Location        string   `json:"location"` // Mumbai, Singapore
	IsActive        bool     `json:"is_active"`
	IsPrimary       bool     `json:"is_primary"`
	Services        []string `json:"services"`
	HealthScore     float64  `json:"health_score"`
	LastHealthCheck time.Time `json:"last_health_check"`
}

// RoutingRule defines how to route requests
type RoutingRule struct {
	UserRegion   string `json:"user_region"`
	TargetRegion string `json:"target_region"`
	RuleType     string `json:"rule_type"` // nearest, failover, primary
	Priority     int    `json:"priority"`
}

func NewMultiRegionService(redis *redis.Client) *MultiRegionService {
	return &MultiRegionService{
		redis: redis,
		regions: []Region{
			{
				ID:        "region_mumbai",
				Name:      "Mumbai (India)",
				Code:      "ap-south-1",
				Location:  "Mumbai",
				IsActive:  true,
				IsPrimary: true,
				Services:  []string{"auth", "ride", "driver", "payment", "matching", "notification", "pricing"},
			},
			{
				ID:        "region_singapore",
				Name:      "Singapore (APAC)",
				Code:      "ap-southeast-1",
				Location:  "Singapore",
				IsActive:  true,
				IsPrimary: false,
				Services:  []string{"auth", "ride", "driver", "payment"},
			},
			{
				ID:        "region_dubai",
				Name:      "Dubai (ME)",
				Code:      "me-central-1",
				Location:  "Dubai",
				IsActive:  false,
				IsPrimary: false,
				Services:  []string{"auth", "ride"},
			},
		},
	}
}

// GetRegionForUser determines which region should handle user request
func (m *MultiRegionService) GetRegionForUser(lat, lng float64, userID string) *Region {
	// Determine user's nearest region based on coordinates
	userRegion := m.determineNearestRegion(lat, lng)

	// Check if that region is healthy
	if userRegion != nil && userRegion.IsActive && m.isRegionHealthy(userRegion.ID) {
		return userRegion
	}

	// Fallback to primary region
	for _, r := range m.regions {
		if r.IsPrimary && r.IsActive {
			return &r
		}
	}

	// Last resort: any active region
	for _, r := range m.regions {
		if r.IsActive {
			return &r
		}
	}

	return nil
}

// determineNearestRegion finds closest region by coordinates
func (m *MultiRegionService) determineNearestRegion(lat, lng float64) *Region {
	// Region coordinates
	regionCoords := map[string]struct{ Lat, Lng float64 }{
		"region_mumbai":    {19.0760, 72.8777},
		"region_singapore": {1.3521, 103.8198},
		"region_dubai":     {25.2048, 55.2708},
	}

	var nearest *Region
	minDist := float64(1e10)

	for _, r := range m.regions {
		if !r.IsActive {
			continue
		}
		coords := regionCoords[r.ID]
		dist := calcDistance(lat, lng, coords.Lat, coords.Lng)
		if dist < minDist {
			minDist = dist
			nearest = &r
		}
	}

	return nearest
}

// calcDistance calculates distance between two points
func calcDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371 // Earth radius in km

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// isRegionHealthy checks region health via Redis
func (m *MultiRegionService) isRegionHealthy(regionID string) bool {
	key := fmt.Sprintf("region:health:%s", regionID)
	score, err := m.redis.Get(context.Background(), key).Float64()
	if err != nil {
		return true // Assume healthy if no data
	}
	return score > 0.5 // Health score threshold
}

// RouteRequest routes a request to appropriate region
func (m *MultiRegionService) RouteRequest(req RoutingRequest) *RoutingDecision {
	region := m.GetRegionForUser(req.Lat, req.Lng, req.UserID)
	if region == nil {
		return &RoutingDecision{
			Success: false,
			Error:   "no healthy region available",
		}
	}

	return &RoutingDecision{
		Success:        true,
		RegionID:       region.ID,
		RegionCode:     region.Code,
		Endpoint:       m.getRegionEndpoint(region.Code),
		Services:       region.Services,
		Latency:        m.estimateLatency(region.ID, req.Lat, req.Lng),
		FailoverRegion: m.getFailoverRegion(region.ID),
	}
}

type RoutingRequest struct {
	UserID string
	Lat    float64
	Lng    float64
	CityID string
}

type RoutingDecision struct {
	Success        bool
	RegionID       string
	RegionCode     string
	Endpoint       string
	Services       []string
	Latency        int // ms
	FailoverRegion string
	Error          string
}

// getRegionEndpoint returns API endpoint for region
func (m *MultiRegionService) getRegionEndpoint(regionCode string) string {
	endpoints := map[string]string{
		"ap-south-1":     "https://api-in.rapido.com",
		"ap-southeast-1": "https://api-sg.rapido.com",
		"me-central-1":   "https://api-ae.rapido.com",
	}
	return endpoints[regionCode]
}

// getFailoverRegion returns backup region
func (m *MultiRegionService) getFailoverRegion(regionID string) string {
	failoverMap := map[string]string{
		"region_mumbai":    "region_singapore",
		"region_singapore": "region_mumbai",
		"region_dubai":     "region_singapore",
	}
	return failoverMap[regionID]
}

// estimateLatency estimates latency based on distance
func (m *MultiRegionService) estimateLatency(regionID string, userLat, userLng float64) int {
	regionCoords := map[string]struct{ Lat, Lng float64 }{
		"region_mumbai":    {19.0760, 72.8777},
		"region_singapore": {1.3521, 103.8198},
		"region_dubai":     {25.2048, 55.2708},
	}

	coords := regionCoords[regionID]
	dist := calcDistance(userLat, userLng, coords.Lat, coords.Lng)

	// Rough estimate: 1ms per 100km + 10ms base
	latency := int(dist*10) + 10
	return latency
}

// HealthCheck performs health check on a region
func (m *MultiRegionService) HealthCheck(regionID string) map[string]interface{} {
	// Simulate health check
	checks := map[string]interface{}{
		"database":      "healthy",
		"redis":         "healthy",
		"kafka":         "healthy",
		"api_gateway":   "healthy",
		"response_time": 45, // ms
	}

	// Calculate overall score
	score := 1.0
	for _, status := range checks {
		if status == "unhealthy" {
			score -= 0.25
		}
	}

	// Store in Redis
	key := fmt.Sprintf("region:health:%s", regionID)
	m.redis.Set(context.Background(), key, score, 30*time.Second)

	return map[string]interface{}{
		"region_id":    regionID,
		"health_score": score,
		"checks":       checks,
		"timestamp":    time.Now(),
	}
}

// GetAllRegions returns all regions and their status
func (m *MultiRegionService) GetAllRegions() []map[string]interface{} {
	var result []map[string]interface{}
	for _, r := range m.regions {
		result = append(result, map[string]interface{}{
			"id":         r.ID,
			"name":       r.Name,
			"code":       r.Code,
			"is_active":  r.IsActive,
			"is_primary": r.IsPrimary,
			"services":   r.Services,
			"health":     m.HealthCheck(r.ID),
		})
	}
	return result
}

// EnableRegion activates a region
func (m *MultiRegionService) EnableRegion(regionID string) {
	for i := range m.regions {
		if m.regions[i].ID == regionID {
			m.regions[i].IsActive = true
			break
		}
	}
}

// DisableRegion deactivates a region (triggers failover)
func (m *MultiRegionService) DisableRegion(regionID string) {
	for i := range m.regions {
		if m.regions[i].ID == regionID {
			m.regions[i].IsActive = false
			break
		}
	}

	// Log failover event
	m.redis.Publish(context.Background(), "region:failover", map[string]interface{}{
		"region_id":       regionID,
		"timestamp":       time.Now(),
		"failover_region": m.getFailoverRegion(regionID),
	})
}

// RegionalRedisKey returns Redis key with region prefix
func (m *MultiRegionService) RegionalRedisKey(regionID, key string) string {
	return fmt.Sprintf("%s:%s", regionID, key)
}

// SyncDataAcrossRegions syncs critical data across regions
func (m *MultiRegionService) SyncDataAcrossRegions(entityType string, entityID string, data interface{}) {
	// Publish to replication topic
	for _, r := range m.regions {
		if !r.IsActive {
			continue
		}

		syncKey := fmt.Sprintf("sync:%s:%s:%s", r.ID, entityType, entityID)
		m.redis.Set(context.Background(), syncKey, data, 24*time.Hour)
	}
}

// GetRegionalConfig returns region-specific configuration
func (m *MultiRegionService) GetRegionalConfig(regionID string) map[string]interface{} {
	configs := map[string]map[string]interface{}{
		"region_mumbai": {
			"pricing_multiplier": 1.0,
			"min_fare":           25,
			"surge_cap":          3.0,
			"timezone":           "Asia/Kolkata",
			"currency":           "INR",
			"payment_methods":    []string{"cash", "upi", "card", "wallet"},
		},
		"region_singapore": {
			"pricing_multiplier": 3.5,
			"min_fare":           5, // SGD
			"surge_cap":          2.5,
			"timezone":           "Asia/Singapore",
			"currency":           "SGD",
			"payment_methods":    []string{"card", "wallet", "paynow"},
		},
	}

	if config, exists := configs[regionID]; exists {
		return config
	}
	return configs["region_mumbai"] // Default
}

// MultiRegionEndpoints returns API endpoints
func GetMultiRegionEndpoints() []map[string]interface{} {
	return []map[string]interface{}{
		{"method": "GET", "path": "/regions", "desc": "List all regions"},
		{"method": "POST", "path": "/regions/route", "desc": "Route request to region"},
		{"method": "GET", "path": "/regions/:id/health", "desc": "Get region health"},
		{"method": "POST", "path": "/admin/regions/:id/enable", "desc": "Enable region"},
		{"method": "POST", "path": "/admin/regions/:id/disable", "desc": "Disable region (failover)"},
		{"method": "GET", "path": "/regions/:id/config", "desc": "Get region config"},
	}
}

var MultiRegionSvc *MultiRegionService

func InitMultiRegionService(redis *redis.Client) {
	MultiRegionSvc = NewMultiRegionService(redis)
}

func GetMultiRegionService() *MultiRegionService {
	return MultiRegionSvc
}
