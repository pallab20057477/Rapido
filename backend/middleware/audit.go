package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuditMiddleware logs all important actions to the audit log
type AuditMiddleware struct {
	sensitiveFields []string
}

// NewAuditMiddleware creates a new audit middleware
func NewAuditMiddleware() *AuditMiddleware {
	return &AuditMiddleware{
		sensitiveFields: []string{"password", "otp", "token", "auth_token", "refresh_token", "credit_card", "cvv"},
	}
}

// LogAction logs an action to the audit log
func (am *AuditMiddleware) LogAction() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip logging for health checks and static files
		if c.Request.URL.Path == "/health" || strings.HasPrefix(c.Request.URL.Path, "/uploads") {
			c.Next()
			return
		}

		// start := time.Now()

		// Capture request body
		var requestBody map[string]interface{}
		if c.Request.Body != nil && c.Request.ContentLength > 0 {
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			json.Unmarshal(bodyBytes, &requestBody)
			// Sanitize sensitive data
			requestBody = am.sanitizeData(requestBody)
		}

		// Process request
		c.Next()

		// Only log write operations and important reads
		method := c.Request.Method
		if method == "GET" && !am.isImportantRead(c.Request.URL.Path) {
			return
		}

		// Get user info from context
		userID, _ := c.Get("userID")
		userRole, _ := c.Get("userRole")
		if userID == nil {
			userID = "anonymous"
		}
		if userRole == nil {
			userRole = "guest"
		}

		// Parse userID to UUID
		var adminID uuid.UUID
		if uidStr, ok := userID.(string); ok && uidStr != "anonymous" {
			adminID, _ = uuid.Parse(uidStr)
		}

		// Parse entity ID if present
		var entityID *uuid.UUID
		if idParam := c.Param("id"); idParam != "" {
			if parsed, err := uuid.Parse(idParam); err == nil {
				entityID = &parsed
			}
		}

		// Create audit log entry
		auditLog := models.AdminActivityLog{
			AdminID:    adminID,
			Action:     method + " " + c.Request.URL.Path,
			EntityType: am.getEntityType(c.Request.URL.Path),
			EntityID:   entityID,
			NewValues:  requestBody,
			IP:         c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
			CreatedAt:  time.Now(),
		}

		// Capture response status
		statusCode := c.Writer.Status()
		auditLog.Description = am.getActionDescription(method, c.Request.URL.Path, statusCode)

		// Async log to database
		go func(logEntry models.AdminActivityLog) {
			if err := database.DB.Create(&logEntry).Error; err != nil {
				utils.Error("Failed to create audit log", zap.Error(err))
			}
		}(auditLog)

		// Log to console using structured logger
		// duration := time.Since(start)
		// statusStr := formatStatus(statusCode)

		// logger := utils.With(
		// 	zap.String("action", "HTTP_REQUEST"),
		// 	zap.String("method", method),
		// 	zap.String("path", c.Request.URL.Path),
		// 	zap.Int("status", statusCode),
		// 	zap.String("client_ip", c.ClientIP()),
		// 	zap.String("user_id", fmt.Sprintf("%v", userID)),
		// 	zap.String("role", fmt.Sprintf("%v", userRole)),
		// 	zap.Duration("duration", duration),
		// )

		if statusCode >= 500 {
			// logger.Error("Request failed", zap.String("response_status", statusStr))
		} else if statusCode >= 400 {
			// logger.Warn("Request error", zap.String("response_status", statusStr))
		} else {
			// logger.Info("Request successful", zap.String("response_status", statusStr))
		}
	}
}

// sanitizeData removes sensitive fields from logged data
func (am *AuditMiddleware) sanitizeData(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	sanitized := make(map[string]interface{})
	for key, value := range data {
		lowerKey := strings.ToLower(key)
		isSensitive := false
		for _, field := range am.sensitiveFields {
			if strings.Contains(lowerKey, field) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			sanitized[key] = "[REDACTED]"
		} else {
			sanitized[key] = value
		}
	}
	return sanitized
}

// isImportantRead determines if a GET request should be logged
func (am *AuditMiddleware) isImportantRead(path string) bool {
	importantPaths := []string{
		"/api/v1/admin",
		"/api/v1/wallet",
		"/api/v1/driver/earnings",
		"/api/v1/rides/history",
		"/api/v1/transactions",
	}

	for _, prefix := range importantPaths {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// getEntityType extracts the entity type from the path
func (am *AuditMiddleware) getEntityType(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 3 {
		entity := parts[len(parts)-1]
		if entity == "" && len(parts) >= 4 {
			entity = parts[len(parts)-2]
		}
		// Remove :id from entity
		entity = strings.Split(entity, ":")[0]
		return entity
	}
	return ""
}

// getActionDescription generates a human-readable description
func (am *AuditMiddleware) getActionDescription(method, path string, statusCode int) string {
	entity := am.getEntityType(path)

	descriptions := map[string]string{
		"POST":   "Created new " + entity,
		"PUT":    "Updated " + entity,
		"PATCH":  "Modified " + entity,
		"DELETE": "Deleted " + entity,
		"GET":    "Retrieved " + entity,
	}

	desc := descriptions[method]
	if desc == "" {
		desc = method + " " + path
	}

	if statusCode >= 400 {
		desc += " (Failed)"
	}

	return desc
}

// LogRideStatusChange logs ride status changes specifically
func LogRideStatusChange(rideID uuid.UUID, oldStatus, newStatus, userID string, reason string) {
	go func() {
		// Parse userID to UUID (or use system UUID for non-logged in)
		adminID, err := uuid.Parse(userID)
		if err != nil {
			adminID = uuid.Nil // system action
		}

		// Use AdminActivityLog for ride status changes
		logEntry := models.AdminActivityLog{
			AdminID:     adminID,
			Action:      "RIDE_STATUS_CHANGE",
			EntityType:  "ride",
			EntityID:    &rideID,
			Description: reason,
			OldValues: map[string]interface{}{
				"status": oldStatus,
			},
			NewValues: map[string]interface{}{
				"status": newStatus,
			},
			CreatedAt: time.Now(),
		}

		if err := database.DB.Create(&logEntry).Error; err != nil {
			utils.Error("Failed to log ride status change", zap.Error(err))
		}
	}()
}

// LogPaymentAttempt logs payment attempts
func LogPaymentAttempt(rideID uuid.UUID, method string, amount float64, success bool, errorMsg string) {
	go func() {
		status := "success"
		if !success {
			status = "failed: " + errorMsg
		}

		logEntry := models.AdminActivityLog{
			AdminID:     uuid.Nil, // system action
			Action:      "PAYMENT_ATTEMPT",
			EntityType:  "payment",
			EntityID:    &rideID,
			Description: fmt.Sprintf("Payment of %s for amount %.2f", method, amount),
			NewValues: map[string]interface{}{
				"ride_id": rideID.String(),
				"method":  method,
				"amount":  amount,
				"status":  status,
			},
			CreatedAt: time.Now(),
		}

		if err := database.DB.Create(&logEntry).Error; err != nil {
			utils.Error("Failed to log payment attempt", zap.Error(err))
		}
	}()
}

// formatStatus formats HTTP status codes with color
func formatStatus(statusCode int) string {
	const (
		colorGreen  = "\033[92m"
		colorYellow = "\033[93m"
		colorRed    = "\033[91m"
		colorReset  = "\033[0m"
	)

	var color string
	if statusCode >= 500 {
		color = colorRed
	} else if statusCode >= 400 {
		color = colorYellow
	} else {
		color = colorGreen
	}

	return fmt.Sprintf("%s%d%s", color, statusCode, colorReset)
}
