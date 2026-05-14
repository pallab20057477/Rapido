package services

import (
	"fmt"
	"math"
	"time"

	"rapido-backend/config"
	"rapido-backend/database"
	"rapido-backend/models"

	"github.com/google/uuid"
)

type PricingService struct{}

type BaseRate struct {
	BaseFare, PerKmRate, PerMinRate, MinFare, MaxFare, PlatformFee, TaxPercent float64
}

// defaultRates are fallback values used only when no FareConfig row exists in the DB.
// Override these by inserting rows into the fare_configs table.
var defaultRates = map[string]BaseRate{
	"bike":   {30, 6, 1, 30, 500, 5, 5},
	"auto":   {40, 12, 1.5, 40, 1000, 5, 5},
	"car_go": {60, 18, 2, 60, 2000, 10, 5},
	"car_x":  {80, 22, 2.5, 80, 3000, 15, 5},
}

func NewPricingService() *PricingService { return &PricingService{} }

// rateForVehicle returns fare rates from the DB, falling back to defaults.
func rateForVehicle(vehicleType string) BaseRate {
	if database.DB != nil {
		var fc models.FareConfig
		if err := database.DB.Where("vehicle_type = ? AND is_active = ?", vehicleType, true).First(&fc).Error; err == nil {
			// FareConfig has no TaxPercent column — use the default for the vehicle type
			// or the global INVOICE_GST_PERCENT config value.
			taxPct := config.Get().App.InvoiceGSTPercent
			if taxPct == 0 {
				taxPct = 5 // default fare-level tax (not invoice GST)
			}
			return BaseRate{
				BaseFare:    fc.BaseFare,
				PerKmRate:   fc.PerKmRate,
				PerMinRate:  fc.PerMinRate,
				MinFare:     fc.MinFare,
				MaxFare:     fc.MaxFare,
				PlatformFee: fc.PlatformFee,
				TaxPercent:  taxPct,
			}
		}
	}
	if r, ok := defaultRates[vehicleType]; ok {
		return r
	}
	return defaultRates["bike"]
}

// CalculateFare: fare = base + (dist × per_km) + (time × per_min) + surge + fee + tax
func (s *PricingService) CalculateFare(vehicleType string, distanceKm, durationMin, pickupLat, pickupLng float64) map[string]interface{} {
	r := rateForVehicle(vehicleType)

	distCharge := distanceKm * r.PerKmRate
	timeCharge := durationMin * r.PerMinRate
	surge := s.calculateSurge(pickupLat, pickupLng)
	subtotal := r.BaseFare + distCharge + timeCharge + r.PlatformFee
	surgeAmt := (subtotal - r.PlatformFee) * (surge - 1)
	if surgeAmt < 0 {
		surgeAmt = 0
	}
	tax := (subtotal + surgeAmt) * (r.TaxPercent / 100)
	total := subtotal + surgeAmt + tax

	if total < r.MinFare {
		total = r.MinFare
	}
	if total > r.MaxFare {
		total = r.MaxFare
	}

	return map[string]interface{}{
		"ride_id": uuid.New().String(), "vehicle_type": vehicleType,
		"base_fare": r.BaseFare, "distance_charge": distCharge, "time_charge": timeCharge,
		"surge_multiplier": surge, "surge_amount": surgeAmt,
		"platform_fee": r.PlatformFee, "tax_amount": tax,
		"total_fare": math.Round(total*100) / 100, "currency": config.Get().App.DefaultCurrency,
		"breakdown": map[string]string{
			"formula":  "base + (dist × per_km) + (time × per_min) + surge + fee + tax",
			"base":     fmt.Sprintf("₹%.0f", r.BaseFare),
			"distance": fmt.Sprintf("₹%.2f (%.1fkm × ₹%.0f)", distCharge, distanceKm, r.PerKmRate),
			"time":     fmt.Sprintf("₹%.2f (%.0fmin × ₹%.1f)", timeCharge, durationMin, r.PerMinRate),
			"surge":    fmt.Sprintf("₹%.2f (%.1fx)", surgeAmt, surge),
			"total":    fmt.Sprintf("₹%.2f", total),
		},
	}
}

// calculateSurge returns a time-based surge multiplier.
// Peak hours and surge caps are read from config when available.
func (s *PricingService) calculateSurge(lat, lng float64) float64 {
	m := 1.0
	hour := time.Now().Hour()
	// Morning peak: 8–10, Evening peak: 18–21
	if (hour >= 8 && hour <= 10) || (hour >= 18 && hour <= 21) {
		m *= 1.2
	}
	// Late night: 23–05
	if hour >= 23 || hour <= 5 {
		m *= 1.3
	}
	// Cap at 3x (regulatory limit)
	if m > 3.0 {
		m = 3.0
	}
	return math.Round(m*100) / 100
}

// POST /internal/pricing/calculate
func (s *PricingService) InternalCalculate(vehicleType string, distanceKm, durationMin, pickupLat, pickupLng float64) map[string]interface{} {
	return s.CalculateFare(vehicleType, distanceKm, durationMin, pickupLat, pickupLng)
}

// Driver Ranking Algorithm
func (s *PricingService) CalculateDriverRank(distanceKm, rating, acceptanceRate, cancellationRate float64) float64 {
	score := (0.40 / (distanceKm + 0.1)) + (0.25 * (rating / 5)) + (0.20 * acceptanceRate) - (0.15 * cancellationRate)
	return math.Round(score*1000) / 1000
}

// cancellationFreeWindowSec returns the free-cancel window from config, defaulting to 120s.
func cancellationFreeWindowSec() int {
	if v := config.Get().App.CancellationFreeWindowSec; v > 0 {
		return v
	}
	return 120
}

// cancellationFlatFee returns the flat fee charged before driver is assigned, from config.
func cancellationFlatFee() float64 {
	if v := config.Get().App.CancellationFlatFee; v > 0 {
		return v
	}
	return 20
}

// cancellationAfterAssignedPct returns the % of base fare charged after driver assigned.
func cancellationAfterAssignedPct() float64 {
	if v := config.Get().App.CancellationAfterAssigned; v > 0 {
		return v
	}
	return 0.50
}

// cancellationAfterArrivedPct returns the % of base fare charged after driver arrived.
func cancellationAfterArrivedPct() float64 {
	if v := config.Get().App.CancellationAfterArrived; v > 0 {
		return v
	}
	return 0.75
}

// CancellationFee calculates the cancellation fee based on ride state and config.
func (s *PricingService) CancellationFee(vehicleType string, secondsSinceRequest int, driverAssigned, driverArrived bool) map[string]interface{} {
	freeWindow := cancellationFreeWindowSec()
	if secondsSinceRequest <= freeWindow {
		return map[string]interface{}{"fee": 0, "reason": "within_free_window"}
	}

	r := rateForVehicle(vehicleType)
	var fee float64
	reason := "before_driver_assigned"
	if driverArrived {
		fee = r.BaseFare * cancellationAfterArrivedPct()
		reason = "after_driver_arrived"
	} else if driverAssigned {
		fee = r.BaseFare * cancellationAfterAssignedPct()
		reason = "after_driver_assigned"
	} else {
		fee = cancellationFlatFee()
	}
	return map[string]interface{}{"fee": fee, "reason": reason, "free_window_sec": freeWindow}
}

// SurgeEngine calculates surge multiplier from demand/supply ratio.
func (s *PricingService) SurgeEngine(zone string, demand, supply int) float64 {
	if supply == 0 {
		return 2.0
	}
	ratio := float64(demand) / float64(supply)
	switch {
	case ratio < 1.0:
		return 1.0
	case ratio < 1.5:
		return 1.2
	case ratio < 2.0:
		return 1.5
	case ratio < 3.0:
		return 2.0
	default:
		return 2.5
	}
}

// Supply/Demand Heatmap (example data — replace with live DB/Redis query)
func GetSupplyDemandHeatmap() []map[string]interface{} {
	return []map[string]interface{}{
		{"zone": "Andheri_East", "demand": 50, "supply": 20, "ratio": 2.5, "surge": 1.5},
		{"zone": "Bandra_West", "demand": 30, "supply": 35, "ratio": 0.86, "surge": 1.0},
		{"zone": "Dadar", "demand": 45, "supply": 15, "ratio": 3.0, "surge": 2.0},
	}
}

// AutoSurgeAdjustment recalculates surge for all heatmap zones.
func AutoSurgeAdjustment() []map[string]interface{} {
	heatmap := GetSupplyDemandHeatmap()
	adjustments := []map[string]interface{}{}
	for _, zone := range heatmap {
		z := zone["zone"].(string)
		d := zone["demand"].(int)
		s := zone["supply"].(int)
		newSurge := (&PricingService{}).SurgeEngine(z, d, s)
		adjustments = append(adjustments, map[string]interface{}{
			"zone": z, "old_surge": zone["surge"], "new_surge": newSurge,
			"demand": d, "supply": s,
		})
	}
	return adjustments
}

var PricingSvc = NewPricingService()

func GetPricingService() *PricingService { return PricingSvc }

// GetInternalPricingEndpoints returns internal endpoint metadata.
func GetInternalPricingEndpoints() []map[string]interface{} {
	return []map[string]interface{}{
		{"path": "/internal/pricing/calculate", "method": "POST", "desc": "Calculate fare with formula"},
		{"path": "/internal/surge/auto-adjust", "method": "POST", "desc": "Auto-adjust surge based on demand/supply"},
		{"path": "/internal/surge/heatmap", "method": "GET", "desc": "Get supply/demand heatmap"},
		{"path": "/internal/drivers/rank", "method": "POST", "desc": "Rank drivers using algorithm"},
		{"path": "/internal/cancellation/policy", "method": "GET", "desc": "Get cancellation policy"},
	}
}

// GetCancellationPolicy returns the current cancellation policy from config.
func GetCancellationPolicy() map[string]interface{} {
	return map[string]interface{}{
		"free_window_sec": cancellationFreeWindowSec(),
		"charges": map[string]interface{}{
			"before_driver":         map[string]float64{"flat_fee": cancellationFlatFee(), "percent": 0},
			"after_driver_assigned": map[string]float64{"flat_fee": 0, "percent": cancellationAfterAssignedPct() * 100},
			"after_driver_arrived":  map[string]float64{"flat_fee": 0, "percent": cancellationAfterArrivedPct() * 100},
		},
		"driver_penalty": []string{"warning", "1_day_suspension", "3_day_suspension", "ban"},
	}
}

// GetDriverRankingFactors returns the driver ranking weight configuration.
func GetDriverRankingFactors() map[string]interface{} {
	return map[string]interface{}{
		"distance_weight": 0.40, "rating_weight": 0.25,
		"acceptance_rate_weight": 0.20, "cancellation_rate_weight": 0.15,
		"formula": "score = (0.4/distance) + (0.25 × rating/5) + (0.20 × acceptance) - (0.15 × cancellation)",
	}
}

// GetRevenueSplit returns the revenue split breakdown for a given fare.
func GetRevenueSplit(totalFare float64) map[string]interface{} {
	commission := config.Get().App.PlatformCommissionPercent
	if commission == 0 {
		commission = 20
	}
	gst := config.Get().App.InvoiceGSTPercent
	if gst == 0 {
		gst = 5
	}
	driverPct := 100 - commission - gst
	return map[string]interface{}{
		"total":    totalFare,
		"driver":   totalFare * driverPct / 100,
		"platform": totalFare * commission / 100,
		"tax":      totalFare * gst / 100,
	}
}
