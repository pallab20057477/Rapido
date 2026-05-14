package services

import (
	"time"
)

// SLATimeoutService manages SLA and timeout configurations
type SLATimeoutService struct {
	timeouts map[string]TimeoutConfig
}

type TimeoutConfig struct {
	Operation    string        `json:"operation"`
	Timeout      time.Duration `json:"timeout"`
	RetryCount   int           `json:"retry_count"`
	RetryDelay   time.Duration `json:"retry_delay"`
	SLA          time.Duration `json:"sla"`
	SLABreachAction string     `json:"sla_breach_action"`
}

func NewSLATimeoutService() *SLATimeoutService {
	service := &SLATimeoutService{
		timeouts: make(map[string]TimeoutConfig),
	}
	service.initializeDefaults()
	return service
}

func (s *SLATimeoutService) initializeDefaults() {
	// Driver-related timeouts
	s.timeouts["driver_accept"] = TimeoutConfig{
		Operation:       "driver_accept",
		Timeout:         30 * time.Second,
		RetryCount:      0,
		SLA:             30 * time.Second,
		SLABreachAction: "reassign_ride",
	}
	s.timeouts["driver_arrive"] = TimeoutConfig{
		Operation:       "driver_arrive",
		Timeout:         10 * time.Minute,
		SLA:             10 * time.Minute,
		SLABreachAction: "notify_support",
	}
	s.timeouts["driver_location_update"] = TimeoutConfig{
		Operation:    "driver_location_update",
		Timeout:      5 * time.Second,
		RetryCount:   3,
		RetryDelay:   1 * time.Second,
		SLA:          5 * time.Second,
	}

	// Payment timeouts
	s.timeouts["payment_init"] = TimeoutConfig{
		Operation:       "payment_init",
		Timeout:         30 * time.Second,
		RetryCount:      3,
		RetryDelay:      5 * time.Second,
		SLA:             30 * time.Second,
		SLABreachAction: "mark_failed_notify_user",
	}
	s.timeouts["payment_confirmation"] = TimeoutConfig{
		Operation:       "payment_confirmation",
		Timeout:         2 * time.Minute,
		RetryCount:      5,
		RetryDelay:      10 * time.Second,
		SLA:             2 * time.Minute,
		SLABreachAction: "manual_reconciliation",
	}

	// Ride timeouts
	s.timeouts["ride_matching"] = TimeoutConfig{
		Operation:       "ride_matching",
		Timeout:         2 * time.Minute,
		SLA:             2 * time.Minute,
		SLABreachAction: "offer_scheduled_ride",
	}
	s.timeouts["ride_cancel"] = TimeoutConfig{
		Operation: "ride_cancel",
		Timeout:   5 * time.Second,
		SLA:       5 * time.Second,
	}

	// OTP timeouts
	s.timeouts["otp_verify"] = TimeoutConfig{
		Operation:    "otp_verify",
		Timeout:      30 * time.Second,
		RetryCount:   0,
		SLA:          30 * time.Second,
	}
	s.timeouts["otp_expire"] = TimeoutConfig{
		Operation: "otp_expire",
		Timeout:   5 * time.Minute,
		SLA:       5 * time.Minute,
	}
}

// GetTimeout returns timeout configuration for operation
func (s *SLATimeoutService) GetTimeout(operation string) TimeoutConfig {
	if config, exists := s.timeouts[operation]; exists {
		return config
	}
	return TimeoutConfig{
		Operation: operation,
		Timeout:   30 * time.Second,
		SLA:       30 * time.Second,
	}
}

// GetAllTimeouts returns all timeout configurations
func (s *SLATimeoutService) GetAllTimeouts() map[string]TimeoutConfig {
	return s.timeouts
}

// UpdateTimeout updates timeout for operation
func (s *SLATimeoutService) UpdateTimeout(operation string, config TimeoutConfig) {
	s.timeouts[operation] = config
}

// CheckSLA checks if operation is within SLA
func (s *SLATimeoutService) CheckSLA(operation string, duration time.Duration) map[string]interface{} {
	config := s.GetTimeout(operation)
	withinSLA := duration <= config.SLA

	status := "within_sla"
	if !withinSLA {
		status = "sla_breached"
	}

	return map[string]interface{}{
		"operation":         operation,
		"duration_ms":       duration.Milliseconds(),
		"sla_ms":            config.SLA.Milliseconds(),
		"within_sla":        withinSLA,
		"status":            status,
		"breach_action":     config.SLABreachAction,
	}
}

// SLADashboard returns SLA metrics dashboard
func (s *SLATimeoutService) SLADashboard() map[string]interface{} {
	return map[string]interface{}{
		"critical_operations": []string{
			"driver_accept",
			"payment_confirmation",
			"ride_matching",
		},
		"timeout_configs": s.timeouts,
		"default_sla_ms":  30000,
		"monitoring": map[string]string{
			"alert_on_breach":    "yes",
			"escalation_channel": "pagerduty",
			"dashboard_url":      "/internal/sla/dashboard",
		},
	}
}

// GetSLASummary returns summary for external reporting
func GetSLASummary() map[string]interface{} {
	return map[string]interface{}{
		"availability_sla":     "99.9%",
		"latency_p99_sla":      "200ms",
		"ride_matching_sla":    "2 minutes",
		"payment_processing_sla": "2 minutes",
		"driver_response_sla":  "30 seconds",
		"penalties": map[string]string{
			"breach_1": "internal_review",
			"breach_3": "executive_escalation",
		},
	}
}

var SLATimeoutSvc *SLATimeoutService

func InitSLATimeoutService() {
	SLATimeoutSvc = NewSLATimeoutService()
}

func GetSLATimeoutService() *SLATimeoutService {
	return SLATimeoutSvc
}
