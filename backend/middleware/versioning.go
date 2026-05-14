package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIVersion represents the API version
type APIVersion string

const (
	APIv1 APIVersion = "v1"
	APIv2 APIVersion = "v2"
)

// CurrentVersion is the latest stable version
const CurrentVersion APIVersion = APIv1

// VersioningMiddleware handles API versioning via header or path
// Supports: Header (Accept-Version: v2), Path (/api/v2/), Query (?version=v2)
func VersioningMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		version := detectVersion(c)
		
		// Store version in context
		c.Set("api_version", version)
		
		// Check if version is supported
		if !isSupportedVersion(version) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Unsupported API version",
				"supported_versions": []string{"v1", "v2"},
				"current_version": CurrentVersion,
			})
			c.Abort()
			return
		}
		
		// Deprecation warning for old versions
		if version != CurrentVersion {
			c.Header("Deprecation", "true")
			c.Header("Sunset", "2025-06-01") // Deprecation date
			c.Header("Link", "</api/v1/>; rel=\"successor-version\"")
		}
		
		c.Next()
	}
}

// detectVersion extracts version from request
func detectVersion(c *gin.Context) APIVersion {
	// 1. Check header (Accept-Version: v2)
	if v := c.GetHeader("Accept-Version"); v != "" {
		return APIVersion(v)
	}
	
	// 2. Check custom header (X-API-Version: v2)
	if v := c.GetHeader("X-API-Version"); v != "" {
		return APIVersion(v)
	}
	
	// 3. Check path (/api/v2/...)
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/api/v2/") {
		return APIv2
	}
	
	// 4. Check query parameter (?version=v2)
	if v := c.Query("version"); v != "" {
		return APIVersion(v)
	}
	
	// Default to v1
	return APIv1
}

// isSupportedVersion checks if version is supported
func isSupportedVersion(version APIVersion) bool {
	switch version {
	case APIv1, APIv2:
		return true
	default:
		return false
	}
}

// GetVersionFromContext retrieves version from gin context
func GetVersionFromContext(c *gin.Context) APIVersion {
	if v, exists := c.Get("api_version"); exists {
		if version, ok := v.(APIVersion); ok {
			return version
		}
	}
	return APIv1
}

// VersionRouter routes to different handlers based on version
type VersionRouter struct {
	handlers map[APIVersion]gin.HandlerFunc
}

// NewVersionRouter creates a version router
func NewVersionRouter() *VersionRouter {
	return &VersionRouter{
		handlers: make(map[APIVersion]gin.HandlerFunc),
	}
}

// Register registers a handler for a version
func (vr *VersionRouter) Register(version APIVersion, handler gin.HandlerFunc) {
	vr.handlers[version] = handler
}

// Handler returns a gin handler that routes to appropriate version
func (vr *VersionRouter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		version := GetVersionFromContext(c)
		
		handler, exists := vr.handlers[version]
		if !exists {
			// Fall back to v1 if version not registered
			handler, exists = vr.handlers[APIv1]
			if !exists {
				c.JSON(http.StatusNotImplemented, gin.H{
					"success": false,
					"error":   "Handler not implemented for this version",
				})
				return
			}
		}
		
		handler(c)
	}
}

// APIVersionInfo provides version information
type APIVersionInfo struct {
	Version          string   `json:"version"`
	Status           string   `json:"status"` // stable, deprecated, beta
	Deprecated       bool     `json:"deprecated"`
	DeprecationDate  string   `json:"deprecation_date,omitempty"`
	SunsetDate       string   `json:"sunset_date,omitempty"`
	BreakingChanges  []string `json:"breaking_changes,omitempty"`
	NewFeatures      []string `json:"new_features,omitempty"`
}

// GetVersionInfo returns information about all API versions
func GetVersionInfo() []APIVersionInfo {
	return []APIVersionInfo{
		{
			Version:         "v1",
			Status:          "stable",
			Deprecated:      false,
			NewFeatures:     []string{"All core features"},
		},
		{
			Version:         "v2",
			Status:          "beta",
			Deprecated:      false,
			NewFeatures:     []string{"ML-based matching", "Advanced fraud detection", "Scheduled rides"},
		},
	}
}
