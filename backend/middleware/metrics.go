package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Request metrics
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint", "status"},
	)

	RequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	// Business metrics
	ActiveRides = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_rides_total",
			Help: "Number of active rides",
		},
	)

	OnlineDrivers = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "online_drivers_total",
			Help: "Number of online drivers by vehicle type",
		},
		[]string{"vehicle_type"},
	)

	RideRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ride_requests_total",
			Help: "Total ride requests",
		},
		[]string{"vehicle_type", "status"},
	)

	PaymentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payments_total",
			Help: "Total payments processed",
		},
		[]string{"method", "status"},
	)

	PaymentAmount = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "payment_amount_inr",
			Help:    "Payment amount in INR",
			Buckets: []float64{50, 100, 200, 500, 1000, 2000, 5000},
		},
		[]string{"method"},
	)

	MatchingDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "driver_matching_duration_seconds",
			Help:    "Time to find and match driver",
			Buckets: []float64{1, 2, 5, 10, 15, 20, 30, 60},
		},
	)

	// Error metrics
	ErrorTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "errors_total",
			Help: "Total errors by type",
		},
		[]string{"type", "endpoint"},
	)

	// Rate limit metrics
	RateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_hits_total",
			Help: "Total rate limit hits",
		},
		[]string{"endpoint", "client_ip"},
	)

	// WebSocket metrics
	WebSocketConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "websocket_connections_active",
			Help: "Active WebSocket connections",
		},
	)

	WebSocketMessages = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "websocket_messages_total",
			Help: "WebSocket messages by type",
		},
		[]string{"message_type", "direction"},
	)
)

// MetricsMiddleware records request metrics
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		// Record metrics
		RequestDuration.WithLabelValues(c.Request.Method, path, status).Observe(duration)
		RequestTotal.WithLabelValues(c.Request.Method, path, status).Inc()

		// Track errors
		if c.Writer.Status() >= 500 {
			ErrorTotal.WithLabelValues("server_error", path).Inc()
		} else if c.Writer.Status() == 429 {
			ErrorTotal.WithLabelValues("rate_limited", path).Inc()
			clientIP := c.ClientIP()
			RateLimitHits.WithLabelValues(path, clientIP).Inc()
		}
	}
}

// RecordRideCreated records ride creation metric
func RecordRideCreated(vehicleType string) {
	RideRequestsTotal.WithLabelValues(vehicleType, "created").Inc()
}

// RecordRideCompleted records ride completion
func RecordRideCompleted(vehicleType string) {
	RideRequestsTotal.WithLabelValues(vehicleType, "completed").Inc()
}

// RecordRideCancelled records ride cancellation
func RecordRideCancelled(vehicleType, reason string) {
	RideRequestsTotal.WithLabelValues(vehicleType, "cancelled_"+reason).Inc()
}

// RecordPayment records payment metric
func RecordPayment(method string, amount float64, success bool) {
	status := "success"
	if !success {
		status = "failed"
	}
	PaymentTotal.WithLabelValues(method, status).Inc()
	PaymentAmount.WithLabelValues(method).Observe(amount)
}

// RecordMatchingTime records driver matching duration
func RecordMatchingTime(duration time.Duration) {
	MatchingDuration.Observe(duration.Seconds())
}

// UpdateActiveRides updates active rides gauge
func UpdateActiveRides(count float64) {
	ActiveRides.Set(count)
}

// UpdateOnlineDrivers updates online drivers gauge
func UpdateOnlineDrivers(vehicleType string, count float64) {
	OnlineDrivers.WithLabelValues(vehicleType).Set(count)
}

// IncWebSocketConnections increments active connections
func IncWebSocketConnections() {
	WebSocketConnections.Inc()
}

// DecWebSocketConnections decrements active connections
func DecWebSocketConnections() {
	WebSocketConnections.Dec()
}

// RecordWebSocketMessage records WebSocket message
func RecordWebSocketMessage(msgType, direction string) {
	WebSocketMessages.WithLabelValues(msgType, direction).Inc()
}
