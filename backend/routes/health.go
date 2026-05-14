package routes

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"rapido-backend/config"
	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HealthStatus represents system health
type HealthStatus struct {
	Status    string                   `json:"status"`
	Version   string                   `json:"version"`
	Timestamp time.Time                `json:"timestamp"`
	Uptime    string                   `json:"uptime"`
	Services  map[string]ServiceHealth `json:"services"`
	System    SystemStats              `json:"system"`
}

// ServiceHealth represents a service health status
type ServiceHealth struct {
	Status  string        `json:"status"`
	Latency time.Duration `json:"latency_ms,omitempty"`
	Error   string        `json:"error,omitempty"`
}

// SystemStats represents system statistics
type SystemStats struct {
	GoVersion     string `json:"go_version"`
	Goroutines    int    `json:"goroutines"`
	MemoryMB      uint64 `json:"memory_mb"`
	MemoryAllocMB uint64 `json:"memory_alloc_mb"`
	NumCPU        int    `json:"num_cpu"`
}

var startTime = time.Now()

// guard to prevent multiple registrations of the same health routes
var healthRoutesRegistered = false

// SetupHealthRoutes configures health check endpoints
func SetupHealthRoutes(router *gin.Engine) {
	if healthRoutesRegistered {
		return
	}
	healthRoutesRegistered = true
	// Basic health check
	router.GET("/health", basicHealthHandler)

	// Detailed health check with dependencies
	router.GET("/health/detailed", detailedHealthHandler)

	// Readiness probe
	router.GET("/ready", readinessHandler)

	// Liveness probe
	router.GET("/live", livenessHandler)

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// basicHealthHandler simple health check
func basicHealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
	})
}

// detailedHealthHandler comprehensive health check
func detailedHealthHandler(c *gin.Context) {
	status := HealthStatus{
		Status:    "healthy",
		Version:   getVersion(),
		Timestamp: time.Now().UTC(),
		Uptime:    time.Since(startTime).String(),
		Services:  make(map[string]ServiceHealth),
		System:    getSystemStats(),
	}

	// Check Database
	dbHealth := checkDatabase()
	status.Services["database"] = dbHealth
	if dbHealth.Status != "healthy" {
		status.Status = "degraded"
	}

	// Check Redis
	redisHealth := checkRedis()
	status.Services["redis"] = redisHealth
	if redisHealth.Status != "healthy" {
		status.Status = "degraded"
	}

	// Check external CRM if configured
	crmHealth := checkExternalCRM()
	status.Services["external_crm"] = crmHealth
	if crmHealth.Status != "healthy" && crmHealth.Status != "disabled" {
		status.Status = "degraded"
	}

	// Set HTTP status based on overall health
	httpStatus := http.StatusOK
	if status.Status == "unhealthy" {
		httpStatus = http.StatusServiceUnavailable
	} else if status.Status == "degraded" {
		httpStatus = http.StatusOK // Still serving but degraded
	}

	c.JSON(httpStatus, status)
}

// readinessHandler checks if app is ready to serve traffic
func readinessHandler(c *gin.Context) {
	// Check critical dependencies
	dbOK := checkDatabase().Status == "healthy"
	redisOK := database.RedisClient == nil || checkRedis().Status == "healthy"

	if dbOK && redisOK {
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
			"checks": gin.H{
				"database": dbOK,
				"redis":    redisOK,
			},
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"checks": gin.H{
				"database": dbOK,
				"redis":    redisOK,
			},
		})
	}
}

// livenessHandler simple liveness check
func livenessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

// checkDatabase checks database connectivity
func checkDatabase() ServiceHealth {
	health := ServiceHealth{Status: "healthy"}

	if database.DB == nil {
		return ServiceHealth{Status: "unhealthy", Error: "database not initialized"}
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sqlDB, err := database.DB.DB()
	if err != nil {
		return ServiceHealth{Status: "unhealthy", Error: err.Error()}
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return ServiceHealth{Status: "unhealthy", Error: err.Error()}
	}

	health.Latency = time.Since(start)
	return health
}

// checkRedis checks Redis connectivity
func checkRedis() ServiceHealth {
	if database.RedisClient == nil {
		return ServiceHealth{Status: "disabled"}
	}

	health := ServiceHealth{Status: "healthy"}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := database.RedisClient.Ping(ctx).Err(); err != nil {
		return ServiceHealth{Status: "unhealthy", Error: err.Error()}
	}

	health.Latency = time.Since(start)
	return health
}

// checkExternalCRM checks external CRM connectivity
func checkExternalCRM() ServiceHealth {
	cfg := config.Get().CRM

	// If no CRM configured, mark as disabled
	if cfg.BaseURL == "" {
		return ServiceHealth{Status: "disabled"}
	}

	health := ServiceHealth{Status: "healthy"}
	start := time.Now()

	// Try to reach CRM health endpoint if available
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.BaseURL+"/health", nil)
	if err != nil {
		return ServiceHealth{Status: "unhealthy", Error: err.Error()}
	}

	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	resp, err := client.Do(req)
	if err != nil {
		// Connection failed, but we still consider CRM "degraded" not fully unhealthy
		// since core app functionality doesn't depend on CRM
		return ServiceHealth{Status: "degraded", Error: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return ServiceHealth{Status: "degraded", Error: fmt.Sprintf("CRM returned %d", resp.StatusCode)}
	}

	health.Latency = time.Since(start)
	return health
}

// getSystemStats returns system statistics
func getSystemStats() SystemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SystemStats{
		GoVersion:     runtime.Version(),
		Goroutines:    runtime.NumGoroutine(),
		MemoryMB:      m.Sys / 1024 / 1024,
		MemoryAllocMB: m.Alloc / 1024 / 1024,
		NumCPU:        runtime.NumCPU(),
	}
}

// getVersion returns app version
func getVersion() string {
	// In production, set this from build flags
	return utils.GetEnv("APP_VERSION", "1.0.0")
}
