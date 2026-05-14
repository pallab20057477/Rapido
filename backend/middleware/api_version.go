package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIVersionMiddleware handles API versioning via headers and URL
func APIVersionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		version := extractVersion(c)
		
		// Set version in context
		c.Set("api_version", version)
		
		// Add deprecation headers if using old version
		if version == "v1" {
			c.Header("Deprecation", "false")
		}
		
		c.Next()
	}
}

// extractVersion determines API version from request
func extractVersion(c *gin.Context) string {
	// Priority 1: Accept header
	accept := c.GetHeader("Accept")
	if strings.Contains(accept, "application/vnd.rapido.v2+json") {
		return "v2"
	}
	if strings.Contains(accept, "application/vnd.rapido.v1+json") {
		return "v1"
	}
	
	// Priority 2: X-API-Version header
	apiVersion := c.GetHeader("X-API-Version")
	if apiVersion == "v2" || apiVersion == "2" {
		return "v2"
	}
	
	// Priority 3: URL path (fallback)
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/api/v2/") {
		return "v2"
	}
	
	// Default to v1
	return "v1"
}

// RequireVersion middleware enforces minimum API version
func RequireVersion(minVersion string) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentVersion := c.GetString("api_version")
		
		if !isVersionCompatible(currentVersion, minVersion) {
			c.JSON(http.StatusNotAcceptable, gin.H{
				"error": "API version not supported",
				"current_version": currentVersion,
				"minimum_required": minVersion,
				"documentation": "https://api.rapido.com/docs/versioning",
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// isVersionCompatible checks if current >= required
func isVersionCompatible(current, required string) bool {
	// Simple version comparison (v2 > v1)
	return current >= required
}

// VersionResponse adds version info to response headers
func VersionResponse() gin.HandlerFunc {
	return func(c *gin.Context) {
		version := c.GetString("api_version")
		c.Header("X-API-Version", version)
		
		// Add sunset header for deprecated versions
		if version == "v1" {
			c.Header("Sunset", "Sat, 01 Jun 2025 00:00:00 GMT")
		}
		
		c.Next()
	}
}
