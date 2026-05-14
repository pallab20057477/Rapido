package services

import (
	"time"

	"github.com/google/uuid"
)

// SupplyBalancingService manages driver supply/demand equilibrium
type SupplyBalancingService struct {
	heatmap    map[string]ZoneData
	surgeZones map[string]float64
	incentives map[string]IncentiveData
}

type ZoneData struct {
	ZoneKey    string  `json:"zone_key"`
	Demand     int     `json:"demand"`
	Supply     int     `json:"supply"`
	Ratio      float64 `json:"ratio"`
	LastUpdate int64   `json:"last_update"`
}

type IncentiveData struct {
	Zone            string  `json:"zone"`
	Amount          float64 `json:"amount"`
	DriversNotified int     `json:"drivers_notified"`
	Timestamp       int64   `json:"timestamp"`
}

func NewSupplyBalancingService() *SupplyBalancingService {
	return &SupplyBalancingService{
		heatmap:    make(map[string]ZoneData),
		surgeZones: make(map[string]float64),
		incentives: make(map[string]IncentiveData),
	}
}

// POST /internal/surge/auto-adjust - Auto-adjust surge based on demand/supply
func (s *SupplyBalancingService) AutoAdjustSurge() map[string]interface{} {
	adjustments := []map[string]interface{}{}

	for zoneKey, data := range s.heatmap {
		if data.Supply == 0 {
			s.surgeZones[zoneKey] = 2.0
			adjustments = append(adjustments, s.createAdjustment(zoneKey, data, 2.0))
			continue
		}

		ratio := float64(data.Demand) / float64(data.Supply)
		var newSurge float64

		switch {
		case ratio < 1.0:
			newSurge = 1.0
		case ratio < 1.5:
			newSurge = 1.2
		case ratio < 2.0:
			newSurge = 1.5
		case ratio < 3.0:
			newSurge = 2.0
		default:
			newSurge = 2.5
		}

		oldSurge := s.surgeZones[zoneKey]
		s.surgeZones[zoneKey] = newSurge

		adjustments = append(adjustments, map[string]interface{}{
			"zone":      zoneKey,
			"old_surge": oldSurge,
			"new_surge": newSurge,
			"demand":    data.Demand,
			"supply":    data.Supply,
			"ratio":     ratio,
			"action":    s.determineAction(ratio),
		})

		// Trigger incentive if shortage
		if ratio > 1.5 {
			s.TriggerDriverIncentive(zoneKey, s.calculateIncentive(ratio))
		}
	}

	return map[string]interface{}{
		"adjustments_made": len(adjustments),
		"adjustments":      adjustments,
		"timestamp":        time.Now().Format(time.RFC3339),
	}
}

func (s *SupplyBalancingService) createAdjustment(zone string, data ZoneData, newSurge float64) map[string]interface{} {
	return map[string]interface{}{
		"zone":      zone,
		"demand":    data.Demand,
		"supply":    data.Supply,
		"new_surge": newSurge,
		"action":    "surge_applied",
	}
}

func (s *SupplyBalancingService) determineAction(ratio float64) string {
	switch {
	case ratio < 1.0:
		return "normal"
	case ratio < 1.5:
		return "mild_surge"
	case ratio < 2.0:
		return "surge_plus_incentive"
	default:
		return "critical_shortage"
	}
}

func (s *SupplyBalancingService) calculateIncentive(ratio float64) float64 {
	if ratio > 3.0 {
		return 100.0
	}
	if ratio > 2.0 {
		return 75.0
	}
	return 50.0
}

// TriggerDriverIncentive - Push notification to drivers in high-demand zones
func (s *SupplyBalancingService) TriggerDriverIncentive(zoneKey string, amount float64) map[string]interface{} {
	incentive := IncentiveData{
		Zone:            zoneKey,
		Amount:          amount,
		DriversNotified: 0, // Would be populated from actual notification
		Timestamp:       time.Now().Unix(),
	}
	s.incentives[zoneKey] = incentive

	return map[string]interface{}{
		"incentive_id":      uuid.New().String(),
		"zone":              zoneKey,
		"amount":            amount,
		"drivers_notified":  15, // Mock
		"expected_response": "20-30%",
		"expires_at":        time.Now().Add(30 * time.Minute).Format(time.RFC3339),
	}
}

// GET /internal/surge/heatmap - Supply/demand heatmap
func (s *SupplyBalancingService) GetHeatmap() []map[string]interface{} {
	result := []map[string]interface{}{}

	for zoneKey, data := range s.heatmap {
		surge := s.surgeZones[zoneKey]
		result = append(result, map[string]interface{}{
			"zone_key":    zoneKey,
			"demand":      data.Demand,
			"supply":      data.Supply,
			"ratio":       data.Ratio,
			"surge":       surge,
			"status":      s.getZoneStatus(data.Ratio),
			"last_update": time.Unix(data.LastUpdate, 0).Format(time.RFC3339),
		})
	}

	return result
}

func (s *SupplyBalancingService) getZoneStatus(ratio float64) string {
	switch {
	case ratio < 0.8:
		return "oversupply"
	case ratio < 1.2:
		return "balanced"
	case ratio < 2.0:
		return "shortage"
	default:
		return "critical"
	}
}

// UpdateZoneData - Real-time update from driver location service
func (s *SupplyBalancingService) UpdateZoneData(zoneKey string, demand, supply int) {
	var ratio float64
	if supply > 0 {
		ratio = float64(demand) / float64(supply)
	} else {
		ratio = 999.0
	}

	s.heatmap[zoneKey] = ZoneData{
		ZoneKey:    zoneKey,
		Demand:     demand,
		Supply:     supply,
		Ratio:      ratio,
		LastUpdate: time.Now().Unix(),
	}
}

// PredictDemand - ML-based prediction (mock)
func (s *SupplyBalancingService) PredictDemand(zoneKey string, targetTime time.Time) map[string]interface{} {
	hour := targetTime.Hour()
	predictedDemand := 30

	// Peak hours prediction
	if (hour >= 8 && hour <= 10) || (hour >= 18 && hour <= 21) {
		predictedDemand = 60
	}

	currentData := s.heatmap[zoneKey]
	confidence := 0.75

	return map[string]interface{}{
		"zone":             zoneKey,
		"current_demand":   currentData.Demand,
		"predicted_demand": predictedDemand,
		"target_time":      targetTime.Format(time.RFC3339),
		"confidence":       confidence,
		"recommendation":   s.getRecommendation(currentData.Supply, predictedDemand),
	}
}

func (s *SupplyBalancingService) getRecommendation(supply, predictedDemand int) string {
	ratio := float64(predictedDemand) / float64(supply)
	if ratio > 1.5 {
		return "pre_position_drivers"
	}
	if ratio > 1.2 {
		return "monitor_closely"
	}
	return "maintain_current"
}

// GetSupplyBalancingSettings
func (s *SupplyBalancingService) GetSettings() map[string]interface{} {
	return map[string]interface{}{
		"auto_surge_enabled":     true,
		"auto_incentive_enabled": true,
		"surge_threshold":        2.0, // ratio > 2.0 triggers surge
		"incentive_threshold":    1.5, // ratio > 1.5 triggers incentive
		"update_interval_sec":    60,
		"zone_granularity":       "1km",
		"max_surge_cap":          3.0,
		"incentive_max":          100.0,
	}
}

// GetIncentiveHistory
func (s *SupplyBalancingService) GetIncentiveHistory() []map[string]interface{} {
	history := []map[string]interface{}{}
	for _, incentive := range s.incentives {
		history = append(history, map[string]interface{}{
			"zone":             incentive.Zone,
			"amount":           incentive.Amount,
			"drivers_notified": incentive.DriversNotified,
			"timestamp":        time.Unix(incentive.Timestamp, 0).Format(time.RFC3339),
		})
	}
	return history
}

// ResetAllSurge - Emergency reset
func (s *SupplyBalancingService) ResetAllSurge() {
	s.surgeZones = make(map[string]float64)
}

// GetSystemStatus
func (s *SupplyBalancingService) GetSystemStatus() map[string]interface{} {
	criticalZones := 0
	shortageZones := 0

	for _, data := range s.heatmap {
		if data.Ratio > 2.0 {
			criticalZones++
		} else if data.Ratio > 1.5 {
			shortageZones++
		}
	}

	return map[string]interface{}{
		"zones_monitored":   len(s.heatmap),
		"critical_zones":    criticalZones,
		"shortage_zones":    shortageZones,
		"balanced_zones":    len(s.heatmap) - criticalZones - shortageZones,
		"active_surge":      len(s.surgeZones),
		"active_incentives": len(s.incentives),
		"auto_balancing":    true,
	}
}

var SupplyBalancer = NewSupplyBalancingService()

func GetSupplyBalancingService() *SupplyBalancingService {
	return SupplyBalancer
}
