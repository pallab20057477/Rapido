package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CORS middleware handles Cross-Origin Resource Sharing
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		allowedOrigins := parseAllowedOrigins(utils.GetEnv("CORS_ALLOW_ORIGIN", ""))

		if origin != "" && !isAllowedOrigin(origin, allowedOrigins) {
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
				return
			}
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
			return
		}

		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Request-ID, Idempotency-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func parseAllowedOrigins(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return false
	}

	for _, allowed := range allowedOrigins {
		if allowed == "*" || strings.EqualFold(origin, allowed) {
			return true
		}
	}

	return false
}

// SecurityHeaders middleware adds security headers
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
		c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'")
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		c.Next()
	}
}

// RequestID middleware adds unique request ID
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		c.Set("requestID", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)

		c.Next()
	}
}

func generateRequestID() string {
	return uuid.NewString()
}

// Logger middleware logs requests
func Logger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		timestamp := param.TimeStamp.Format("2006/01/02 - 15:04:05")
		statusColor := "\033[0m" // default (reset)
		switch {
		case param.StatusCode >= 200 && param.StatusCode < 300:
			statusColor = "\033[32m" // green
		case param.StatusCode >= 300 && param.StatusCode < 400:
			statusColor = "\033[36m" // cyan
		case param.StatusCode >= 400 && param.StatusCode < 500:
			statusColor = "\033[33m" // yellow
		case param.StatusCode >= 500:
			statusColor = "\033[31m" // red
		}
		resetColor := "\033[0m"
		message := fmt.Sprintf("[GIN] %s | %s%3d%s | %13v | %s | %s %q",
			timestamp,
			statusColor,
			param.StatusCode,
			resetColor,
			param.Latency,
			param.ClientIP,
			param.Method,
			param.Path,
		)

		if param.ErrorMessage != "" {
			return message + " | " + param.ErrorMessage + "\n"
		}

		return message + "\n"
	})
}
