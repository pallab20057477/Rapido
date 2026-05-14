package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"rapido-backend/config"
	"rapido-backend/database"
	"rapido-backend/middleware"
	"rapido-backend/routes"
	"rapido-backend/services"
	"rapido-backend/utils"
	"rapido-backend/websocket"
	"rapido-backend/workers"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func main() {
	// Initialize structured logger first
	utils.InitLogger(utils.GetEnv("APP_ENV", "production"))
	defer utils.Sync()

	logger := utils.With(zap.String("component", "main"))
	logger.Info("Starting Rapido Backend", zap.String("version", utils.GetEnv("APP_VERSION", "1.0.0")))

	// Load configuration
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		logger.Fatal("Invalid production configuration", zap.Error(err))
	}

	// Set Gin mode with a production-safe default
	switch cfg.Server.Mode {
	case "debug":
		gin.SetMode(gin.DebugMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.ReleaseMode)
		// Disable Gin's default logger in production (we use structured logging)
		gin.DisableConsoleColor()
	}

	// Connect to database
	logger.Info("Connecting to database...")
	db, err := database.Connect(cfg)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	logger.Info("Database connected successfully")

	// Run migrations
	logger.Info("Running database migrations...")
	if err := database.Migrate(db); err != nil {
		logger.Fatal("Failed to migrate database", zap.Error(err))
	}
	logger.Info("Database migrations completed")

	// Ensure the configured admin account exists for immediate admin login
	if err := services.EnsureAdminUser(cfg); err != nil {
		logger.Fatal("Failed to bootstrap admin user", zap.Error(err))
	}
	logger.Info("Admin bootstrap completed")

	// Connect to Redis
	logger.Info("Connecting to Redis...")
	if _, err := database.ConnectRedis(cfg); err != nil {
		if isProductionEnvironment(cfg.App.Environment) {
			logger.Fatal("Failed to connect to Redis in production", zap.Error(err))
		}
		logger.Warn("Failed to connect to Redis, continuing without caching", zap.Error(err))
	} else {
		logger.Info("Redis connected successfully")
	}

	// Initialize worker pool
	workers.InitWorkerPool()
	defer workers.StopWorkerPool()
	logger.Info("Background workers started")

	// Initialize SMS service
	services.InitSMSService()
	logger.Info("SMS service initialized")

	// Initialize FCM service
	if err := services.InitFCMService(); err != nil {
		logger.Warn("Failed to initialize FCM service", zap.Error(err))
	} else {
		logger.Info("FCM service initialized")
	}

	// Set up callbacks to avoid import cycles between workers and services
	// FCM notification callback
	workers.FCMNotifyCallback = func(userID uuid.UUID, title, body string, data map[string]interface{}) error {
		if fcmService := services.GetFCMService(); fcmService != nil && fcmService.IsEnabled() {
			return fcmService.SendPushNotification(userID, title, body, data)
		}
		return nil
	}
	// CRM sync callback
	services.CRMSyncCallback = func(event, entityType, entityID string, data map[string]interface{}) {
		if workers.WorkerPoolInstance != nil {
			workers.WorkerPoolInstance.EnqueueCRMSync(event, entityType, entityID, data)
		}
	}
	// Job submission callback
	services.SubmitJobCallback = func(jobType string, payload interface{}) error {
		if workers.WorkerPoolInstance != nil {
			return workers.SubmitJob(jobType, payload)
		}
		return nil
	}
	logger.Info("Worker-service callbacks initialized")

	// Initialize surge pricing service
	surgeService := services.NewSurgePricingService()
	surgeService.StartSurgeMonitoring()
	logger.Info("Dynamic surge pricing monitoring started")

	// Initialize distributed lock manager
	services.InitLockManager()
	logger.Info("Distributed lock manager initialized")

	// Initialize fraud detection
	services.InitFraudDetection()
	logger.Info("Fraud detection initialized")

	// Initialize WebSocket scaling for multi-server support
	websocket.InitWebSocketScaling(fmt.Sprintf("ws-%d", time.Now().UnixNano()))
	websocket.InitHealthChecker()
	logger.Info("WebSocket scaling initialized")

	// Initialize ride timeout service
	services.InitTimeoutService()
	logger.Info("Ride timeout service initialized")

	// Initialize circuit breakers for external APIs
	utils.InitCircuitBreakers()
	logger.Info("Circuit breakers initialized")

	// Create router with recovery middleware
	r := gin.New()
	r.Use(gin.Recovery())

	// Set trusted proxies (empty = trust none, use X-Forwarded-For carefully)
	if err := r.SetTrustedProxies([]string{}); err != nil {
		logger.Warn("Failed to set trusted proxies", zap.Error(err))
	}

	// Add middleware (order matters!)
	r.Use(middleware.RequestID())                // 1. Generate request ID
	r.Use(middleware.MetricsMiddleware())        // 2. Record metrics
	r.Use(middleware.Logger())                   // 3. Request logging in Gin console format
	r.Use(middleware.CORS())                     // 4. CORS
	r.Use(middleware.SecurityHeaders())          // 5. Security headers
	r.Use(middleware.TokenBlacklistMiddleware()) // 6. JWT blacklist check

	// Add audit logging middleware
	auditMiddleware := middleware.NewAuditMiddleware()
	r.Use(auditMiddleware.LogAction())

	// Setup health check routes (before main routes)
	routes.SetupHealthRoutes(r)

	// Setup main API routes
	routes.SetupRoutes(r)

	// Start server with graceful shutdown
	listener, addr, err := createServerListener(cfg.Server.Port)
	if err != nil {
		logger.Fatal("Failed to determine server port", zap.Error(err))
	}
	if addr != ":"+cfg.Server.Port {
		logger.Warn("Configured server port is busy, using fallback port", zap.String("configured_port", cfg.Server.Port), zap.String("fallback_address", addr))
	}
	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Server starting", zap.String("address", addr), zap.String("mode", gin.Mode()))
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutdown signal received, gracefully stopping...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	workers.StopWorkerPool()
	logger.Info("Server gracefully stopped")
}

func createServerListener(configuredPort string) (net.Listener, string, error) {
	port, err := strconv.Atoi(configuredPort)
	if err != nil || port <= 0 {
		port = 8080
	}

	for candidate := port; candidate <= port+20; candidate++ {
		addr := fmt.Sprintf(":%d", candidate)
		ln, listenErr := net.Listen("tcp", addr)
		if listenErr != nil {
			continue
		}
		return ln, addr, nil
	}

	return nil, "", fmt.Errorf("no available port found in range %d-%d", port, port+20)
}

func isProductionEnvironment(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "production", "staging":
		return true
	default:
		return false
	}
}
