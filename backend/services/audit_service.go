package services

import (
	"time"

	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AuditService handles audit logging
type AuditService struct {
	db *gorm.DB
}

// NewAuditService creates service
func NewAuditService() *AuditService {
	return &AuditService{
		db: database.DB,
	}
}

// Log creates an audit log entry
func (s *AuditService) Log(
	userID *uuid.UUID,
	userType string,
	action string,
	entityType string,
	entityID string,
	oldValues map[string]interface{},
	newValues map[string]interface{},
	ipAddress string,
	userAgent string,
) (*models.AuditLog, error) {
	log := &models.AuditLog{
		UserID:     userID,
		UserType:   userType,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		OldValues:  oldValues,
		NewValues:  newValues,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Status:     "success",
		Severity:   "info",
	}

	// Use separate goroutine to not block main flow
	go func() {
		if err := s.db.Create(log).Error; err != nil {
			utils.Error("Failed to create audit log", zap.Error(err))
		}
	}()

	return log, nil
}

// LogAsync creates audit log asynchronously (fire and forget)
func (s *AuditService) LogAsync(
	userID *uuid.UUID,
	userType string,
	action string,
	entityType string,
	entityID string,
	oldValues map[string]interface{},
	newValues map[string]interface{},
) {
	go func() {
		log := &models.AuditLog{
			UserID:     userID,
			UserType:   userType,
			Action:     action,
			EntityType: entityType,
			EntityID:   entityID,
			OldValues:  oldValues,
			NewValues:  newValues,
			Status:     "success",
			Severity:   "info",
		}
		
		if err := s.db.Create(log).Error; err != nil {
			utils.Error("Failed to create audit log", zap.Error(err))
		}
	}()
}

// LogSecurity logs security-related events
func (s *AuditService) LogSecurity(
	userID *uuid.UUID,
	userType string,
	action string,
	ipAddress string,
	status string,
	errorMsg string,
) {
	severity := "warning"
	if status == "failed" {
		severity = "critical"
	}

	log := &models.AuditLog{
		UserID:       userID,
		UserType:     userType,
		Action:       action,
		EntityType:   models.AuditEntityUser,
		IPAddress:    ipAddress,
		Status:       status,
		ErrorMessage: errorMsg,
		Severity:     severity,
	}

	go func() {
		if err := s.db.Create(log).Error; err != nil {
			utils.Error("Failed to create security audit log", zap.Error(err))
		}
	}()
}

// LogPiiAccess logs PII data access
func (s *AuditService) LogPiiAccess(
	userID uuid.UUID,
	userType string,
	entityType string,
	entityID string,
	fieldsAccessed []string,
	reason string,
) {
	log := &models.AuditLog{
		UserID:     &userID,
		UserType:   userType,
		Action:     models.AuditActionPiiAccessed,
		EntityType: entityType,
		EntityID:   entityID,
		NewValues: map[string]interface{}{
			"fields_accessed": fieldsAccessed,
			"reason":          reason,
		},
		Severity: "warning",
	}

	go func() {
		if err := s.db.Create(log).Error; err != nil {
			utils.Error("Failed to create PII audit log", zap.Error(err))
		}
	}()
}

// GetAuditLogs retrieves audit logs with filtering
func (s *AuditService) GetAuditLogs(
	userID *uuid.UUID,
	entityType string,
	action string,
	startDate, endDate time.Time,
	page, limit int,
) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	offset := (page - 1) * limit

	query := s.db.Model(&models.AuditLog{})

	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}

	if entityType != "" {
		query = query.Where("entity_type = ?", entityType)
	}

	if action != "" {
		query = query.Where("action = ?", action)
	}

	if !startDate.IsZero() && !endDate.IsZero() {
		query = query.Where("created_at BETWEEN ? AND ?", startDate, endDate)
	}

	query.Count(&total)

	result := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs)

	return logs, total, result.Error
}

// GetCriticalEvents gets critical security events
func (s *AuditService) GetCriticalEvents(
	startDate, endDate time.Time,
	page, limit int,
) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	offset := (page - 1) * limit

	query := s.db.Model(&models.AuditLog{}).
		Where("severity = ?", "critical")

	if !startDate.IsZero() && !endDate.IsZero() {
		query = query.Where("created_at BETWEEN ? AND ?", startDate, endDate)
	}

	query.Count(&total)

	result := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs)

	return logs, total, result.Error
}

// CleanupOldLogs removes logs older than retention period
func (s *AuditService) CleanupOldLogs(retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	
	result := s.db.Where("created_at < ? AND severity != ?", cutoff, "critical").
		Delete(&models.AuditLog{})
	
	if result.Error != nil {
		return result.Error
	}

	utils.Info("Cleaned up old audit logs", zap.Int64("deleted", result.RowsAffected))
	return nil
}

// GetAuditStats gets statistics for admin dashboard
func (s *AuditService) GetAuditStats(days int) map[string]interface{} {
	since := time.Now().AddDate(0, 0, -days)

	var stats struct {
		TotalEvents     int64
		FailedEvents    int64
		SecurityEvents  int64
		PiiAccessEvents int64
	}

	s.db.Model(&models.AuditLog{}).Where("created_at > ?", since).Count(&stats.TotalEvents)
	s.db.Model(&models.AuditLog{}).Where("created_at > ? AND status = ?", since, "failed").Count(&stats.FailedEvents)
	s.db.Model(&models.AuditLog{}).Where("created_at > ? AND action IN ?", since, []string{
		models.AuditActionUserLogin,
		models.AuditActionPasswordChanged,
	}).Count(&stats.SecurityEvents)
	s.db.Model(&models.AuditLog{}).Where("created_at > ? AND action = ?", since, models.AuditActionPiiAccessed).Count(&stats.PiiAccessEvents)

	return map[string]interface{}{
		"total_events":      stats.TotalEvents,
		"failed_events":     stats.FailedEvents,
		"security_events":   stats.SecurityEvents,
		"pii_access_events": stats.PiiAccessEvents,
		"period_days":       days,
	}
}

// Global instance
var AuditSvc *AuditService

// InitAuditService initializes service
func InitAuditService() {
	AuditSvc = NewAuditService()
	
	// Start cleanup job for logs older than 1 year
	go func() {
		for {
			time.Sleep(24 * time.Hour)
			AuditSvc.CleanupOldLogs(365) // Keep 1 year
		}
	}()
}
