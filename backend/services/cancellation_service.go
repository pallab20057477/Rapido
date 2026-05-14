package services

import (
	"fmt"
	"time"
	"github.com/google/uuid"
)

// CancellationService handles cancellation policy enforcement
type CancellationService struct {
	penalties map[string][]PenaltyRecord
}

type PenaltyRecord struct {
	Timestamp     int64   `json:"timestamp"`
	Reason        string  `json:"reason"`
	Fee           float64 `json:"fee"`
	PenaltyLevel  int     `json:"penalty_level"`
}

func NewCancellationService() *CancellationService {
	return &CancellationService{
		penalties: make(map[string][]PenaltyRecord),
	}
}

// CancellationPolicyEngine - Main entry point
// POST /internal/cancellation/policy/apply
func (s *CancellationService) ApplyPolicy(
	rideID, userID, userType, vehicleType string,
	secondsSinceRequest int,
	driverAssigned, driverArrived bool,
	reason string,
) map[string]interface{} {
	
	// Validate cancellation is allowed
	valid, msg := s.ValidateCancellation(secondsSinceRequest)
	if !valid {
		return map[string]interface{}{"allowed": false, "error": msg}
	}
	
	// Calculate fee based on policy
	fee := s.CalculateFee(vehicleType, secondsSinceRequest, driverAssigned, driverArrived)
	
	// Apply penalty for driver cancellations
	var penalty map[string]interface{}
	if userType == "driver" {
		penalty = s.ApplyDriverPenalty(userID, reason)
	}
	
	// Calculate refund (if applicable)
	refundAmount := s.CalculateRefund(fee["fee"].(float64), secondsSinceRequest, driverAssigned)
	
	return map[string]interface{}{
		"allowed":         true,
		"ride_id":         rideID,
		"user_id":         userID,
		"user_type":       userType,
		"fee":             fee["fee"],
		"fee_reason":      fee["reason"],
		"penalty":         penalty,
		"refund_amount":   refundAmount,
		"cancellation_id": uuid.New().String(),
		"timestamp":       time.Now().Format(time.RFC3339),
	}
}

// ValidateCancellation - Check if cancellation is allowed
func (s *CancellationService) ValidateCancellation(secondsSinceRequest int) (bool, string) {
	// Can't cancel if negative time
	if secondsSinceRequest < 0 {
		return false, "Invalid cancellation time"
	}
	return true, "Cancellation allowed"
}

// CalculateFee - Calculate cancellation fee based on timing
func (s *CancellationService) CalculateFee(
	vehicleType string,
	secondsSinceRequest int,
	driverAssigned, driverArrived bool,
) map[string]interface{} {
	
	// FREE CANCELLATION within 2 minutes (120 seconds)
	if secondsSinceRequest <= 120 {
		return map[string]interface{}{
			"fee":     0.0,
			"reason":  "within_free_window",
			"message": "Free cancellation within 2 minutes",
		}
	}
	
	// Base rates for fee calculation
	baseRates := map[string]float64{"bike": 30, "auto": 40, "cab": 60}
	base := baseRates[vehicleType]
	
	var fee float64
	var reason string
	var message string
	
	// After 2 minutes - graduated fee structure
	if driverArrived {
		// Driver already arrived - 75% of base fare
		fee = base * 0.75
		reason = "after_driver_arrived"
		message = fmt.Sprintf("Driver arrived - cancellation fee: ₹%.2f (75%% of base fare)", fee)
	} else if driverAssigned {
		// Driver assigned but not arrived - 50% of base fare
		fee = base * 0.50
		reason = "after_driver_assigned"
		message = fmt.Sprintf("Driver assigned - cancellation fee: ₹%.2f (50%% of base fare)", fee)
	} else {
		// No driver assigned yet - flat ₹20 fee
		fee = 20.0
		reason = "before_driver_assigned"
		message = fmt.Sprintf("Cancellation fee: ₹%.2f (after 2 min free window)", fee)
	}
	
	return map[string]interface{}{
		"fee":     fee,
		"reason":  reason,
		"message": message,
	}
}

// ApplyDriverPenalty - Penalty system for driver cancellations
func (s *CancellationService) ApplyDriverPenalty(driverID, reason string) map[string]interface{} {
	records := s.penalties[driverID]
	count := len(records)
	
	var penalty string
	var nextPenalty string
	var suspensionHours int
	
	// Progressive penalty system
	switch count {
	case 0:
		penalty = "warning"
		nextPenalty = "1 day suspension"
		suspensionHours = 0
	case 1:
		penalty = "1_day_suspension"
		nextPenalty = "3 day suspension"
		suspensionHours = 24
	case 2:
		penalty = "3_day_suspension"
		nextPenalty = "permanent ban"
		suspensionHours = 72
	default:
		penalty = "permanent_ban"
		nextPenalty = "N/A"
		suspensionHours = -1
	}
	
	// Record penalty
	record := PenaltyRecord{
		Timestamp:    time.Now().Unix(),
		Reason:       reason,
		Fee:          0, // No fee for drivers
		PenaltyLevel: count + 1,
	}
	s.penalties[driverID] = append(records, record)
	
	return map[string]interface{}{
		"driver_id":        driverID,
		"cancellation_count": count + 1,
		"penalty":          penalty,
		"suspension_hours": suspensionHours,
		"next_penalty":     nextPenalty,
		"effective_until":  time.Now().Add(time.Duration(suspensionHours) * time.Hour).Format(time.RFC3339),
	}
}

// CalculateRefund - Calculate refund for pre-paid rides
func (s *CancellationService) CalculateRefund(fee float64, secondsSinceRequest int, driverAssigned bool) float64 {
	// If no fee charged, full refund
	if fee == 0 {
		return 100.0 // Full refund percentage
	}
	
	// After driver assigned, lower refund
	if driverAssigned {
		return 25.0 // 25% refund
	}
	
	// Before driver assigned, partial refund
	return 50.0 // 50% refund
}

// GetCancellationPolicy - Full policy details
func (s *CancellationService) GetPolicy() map[string]interface{} {
	return map[string]interface{}{
		"free_cancellation_window_sec": 120,
		"free_cancellation_description": "Full refund within 2 minutes of booking",
		"charges": map[string]interface{}{
			"before_driver_assigned": map[string]interface{}{
				"flat_fee_inr":    20,
				"percentage":      0,
				"description":     "₹20 flat fee after 2 minutes, no driver assigned",
			},
			"after_driver_assigned": map[string]interface{}{
				"flat_fee_inr":    0,
				"percentage":      50,
				"description":     "50% of base fare when driver assigned",
			},
			"after_driver_arrived": map[string]interface{}{
				"flat_fee_inr":    0,
				"percentage":      75,
				"description":     "75% of base fare when driver arrived",
			},
		},
		"driver_cancellation_penalties": []map[string]interface{}{
			{"offense": 1, "penalty": "warning", "suspension": "0 hours"},
			{"offense": 2, "penalty": "1_day_suspension", "suspension": "24 hours"},
			{"offense": 3, "penalty": "3_day_suspension", "suspension": "72 hours"},
			{"offense": 4, "penalty": "permanent_ban", "suspension": "indefinite"},
		},
		"no_driver_found": map[string]interface{}{
			"rider_charge":   0,
			"auto_rematch":   true,
			"max_wait_sec":   120,
		},
		"refund_policy": map[string]interface{}{
			"within_2min":            "100% refund",
			"after_2min_no_driver":    "100% refund",
			"after_2min_with_driver": "Prorated refund minus cancellation fee",
		},
	}
}

// GetDriverCancellationStats
func (s *CancellationService) GetDriverStats(driverID string) map[string]interface{} {
	records := s.penalties[driverID]
	count := len(records)
	
	var last30Days int
	cutoff := time.Now().AddDate(0, 0, -30).Unix()
	for _, r := range records {
		if r.Timestamp > cutoff {
			last30Days++
		}
	}
	
	return map[string]interface{}{
		"driver_id":            driverID,
		"total_cancellations":  count,
		"last_30_days":         last30Days,
		"current_penalty":      s.getCurrentPenalty(count),
		"risk_level":           s.getRiskLevel(count, last30Days),
	}
}

func (s *CancellationService) getCurrentPenalty(count int) string {
	switch {
	case count == 0:
		return "none"
	case count == 1:
		return "warning"
	case count == 2:
		return "1_day_suspension"
	case count == 3:
		return "3_day_suspension"
	default:
		return "permanent_ban"
	}
}

func (s *CancellationService) getRiskLevel(total, last30 int) string {
	if total >= 4 || last30 >= 2 {
		return "high"
	}
	if total >= 2 || last30 >= 1 {
		return "medium"
	}
	return "low"
}

// ProcessRefund - Initiate refund
func (s *CancellationService) ProcessRefund(rideID string, amount float64, reason string) map[string]interface{} {
	return map[string]interface{}{
		"refund_id":       uuid.New().String(),
		"ride_id":         rideID,
		"amount":          amount,
		"reason":          reason,
		"status":          "processing",
		"eta":             "3-5 business days",
		"processed_at":    time.Now().Format(time.RFC3339),
	}
}

// GetCancellationReasons - Valid cancellation reasons
func (s *CancellationService) GetReasons() []map[string]interface{} {
	return []map[string]interface{}{
		{"code": "change_of_plan", "label": "Change of plan", "applies_to": "rider"},
		{"code": "wrong_pickup", "label": "Wrong pickup location", "applies_to": "rider"},
		{"code": "found_alternative", "label": "Found alternative ride", "applies_to": "rider"},
		{"code": "vehicle_issue", "label": "Vehicle issue", "applies_to": "driver"},
		{"code": "emergency", "label": "Emergency", "applies_to": "all"},
		{"code": "rider_no_show", "label": "Rider didn't show", "applies_to": "driver"},
		{"code": "driver_no_show", "label": "Driver didn't show", "applies_to": "rider"},
		{"code": "other", "label": "Other", "applies_to": "all"},
	}
}

var CancellationSvc = NewCancellationService()

func GetCancellationService() *CancellationService {
	return CancellationSvc
}
