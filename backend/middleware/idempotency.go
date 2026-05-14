package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/context"
)

type IdempotencyMiddleware struct {
	redis *redis.Client
}

type IdempotencyResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Timestamp  time.Time         `json:"timestamp"`
}

func NewIdempotencyMiddleware(redisClient *redis.Client) *IdempotencyMiddleware {
	return &IdempotencyMiddleware{
		redis: redisClient,
	}
}

// GenerateKey creates an idempotency key from the request
func (im *IdempotencyMiddleware) GenerateKey(c *gin.Context) string {
	// Get idempotency key from header
	key := c.GetHeader("Idempotency-Key")
	if key == "" {
		return ""
	}

	// Add user ID and endpoint for uniqueness
	userID := c.GetString("userID")
	if userID == "" {
		userID = c.GetString("user_id")
	}
	if userID == "" {
		userID = "anonymous"
	}

	endpoint := c.Request.URL.Path
	method := c.Request.Method

	// Create composite key
	composite := fmt.Sprintf("%s:%s:%s:%s", key, userID, method, endpoint)

	// Hash to avoid key length issues
	hash := sha256.Sum256([]byte(composite))
	return hex.EncodeToString(hash[:])
}

// CheckExisting checks if an idempotent request already exists
func (im *IdempotencyMiddleware) CheckExisting(c *gin.Context, key string) (*IdempotencyResponse, error) {
	if key == "" {
		return nil, nil
	}

	ctx := context.Background()

	// Get existing response from Redis
	data, err := im.redis.Get(ctx, "idempotency:"+key).Result()
	if err == redis.Nil {
		return nil, nil // No existing request
	}
	if err != nil {
		return nil, fmt.Errorf("failed to check idempotency: %w", err)
	}

	// Parse stored response
	var response IdempotencyResponse
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		return nil, fmt.Errorf("failed to parse idempotency response: %w", err)
	}

	return &response, nil
}

// StoreResponse stores the response for idempotency
func (im *IdempotencyMiddleware) StoreResponse(c *gin.Context, key string, statusCode int, body []byte) error {
	if key == "" {
		return nil
	}

	ctx := context.Background()

	// Extract relevant headers
	headers := make(map[string]string)
	relevantHeaders := []string{"Content-Type", "Content-Length", "Cache-Control"}

	for _, header := range relevantHeaders {
		if value := c.GetHeader(header); value != "" {
			headers[header] = value
		}
	}

	response := IdempotencyResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       string(body),
		Timestamp:  time.Now(),
	}

	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal idempotency response: %w", err)
	}

	// Store for 24 hours
	return im.redis.Set(ctx, "idempotency:"+key, data, 24*time.Hour).Err()
}

// Middleware creates the idempotency middleware
func (im *IdempotencyMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only apply to POST, PUT, PATCH requests
		if !im.isIdempotentMethod(c.Request.Method) {
			c.Next()
			return
		}

		key := im.GenerateKey(c)
		if key == "" {
			// No idempotency key provided, continue normally
			c.Next()
			return
		}

		// Check for existing request
		existing, err := im.CheckExisting(c, key)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to check idempotency",
				"code":  "IDEMPOTENCY_ERROR",
			})
			c.Abort()
			return
		}

		if existing != nil {
			// Return existing response
			for key, value := range existing.Headers {
				c.Header(key, value)
			}
			c.Data(existing.StatusCode, "application/json", []byte(existing.Body))
			c.Abort()
			return
		}

		// Store key in context for later use
		c.Set("idempotency_key", key)

		// Capture response
		c.Writer = &idempotencyResponseWriter{
			ResponseWriter: c.Writer,
			context:        c,
			middleware:     im,
		}

		c.Next()
	}
}

// isIdempotentMethod checks if the method should be idempotent
func (im *IdempotencyMiddleware) isIdempotentMethod(method string) bool {
	return method == "POST" || method == "PUT" || method == "PATCH"
}

// idempotencyResponseWriter captures response for idempotency
type idempotencyResponseWriter struct {
	gin.ResponseWriter
	context    *gin.Context
	middleware *IdempotencyMiddleware
	body       []byte
	written    bool
}

func (w *idempotencyResponseWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.body = data
		w.written = true
	}
	return w.ResponseWriter.Write(data)
}

func (w *idempotencyResponseWriter) WriteHeader(statusCode int) {
	if !w.written {
		// Store response for idempotency
		key, exists := w.context.Get("idempotency_key")
		if exists {
			w.middleware.StoreResponse(w.context, key.(string), statusCode, w.body)
		}
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

// Helper function to validate idempotency key format
func ValidateIdempotencyKey(key string) bool {
	if len(key) < 8 || len(key) > 255 {
		return false
	}

	// Check for valid characters (alphanumeric, hyphens, underscores)
	for _, char := range key {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return false
		}
	}

	return true
}

// GetIdempotencyKey extracts and validates the idempotency key
func GetIdempotencyKey(c *gin.Context) (string, error) {
	key := c.GetHeader("Idempotency-Key")
	if key == "" {
		return "", nil // Optional header
	}

	if !ValidateIdempotencyKey(key) {
		return "", fmt.Errorf("invalid idempotency key format")
	}

	return key, nil
}
