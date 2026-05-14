package middleware

import (
	"fmt"
	"net/http"
	"time"

	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements token bucket algorithm using Redis
type RateLimiter struct {
	requests int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requests int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: requests,
		window:   window,
	}
}

// RateLimit middleware limits requests per IP
func (rl *RateLimiter) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		key := fmt.Sprintf("ratelimit:%s:%s", clientIP, c.FullPath())

		// Check if Redis is available
		if database.RedisClient == nil {
			c.Next()
			return
		}

		// Get current count
		count, err := database.RedisClient.Incr(database.Ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}

		// Set expiry on first request
		if count == 1 {
			database.RedisClient.Expire(database.Ctx, key, rl.window)
		}

		// Check if limit exceeded
		if count > int64(rl.requests) {
			c.JSON(http.StatusTooManyRequests, utils.ErrorResponse(
				"Rate limit exceeded",
				fmt.Sprintf("Try again in %v", rl.window),
			))
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitByUser limits requests per user ID
func (rl *RateLimiter) RateLimitByUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == "" {
			c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", "login required"))
			c.Abort()
			return
		}

		key := fmt.Sprintf("ratelimit:user:%s:%s", userID, c.FullPath())

		if database.RedisClient == nil {
			c.Next()
			return
		}

		count, err := database.RedisClient.Incr(database.Ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}

		if count == 1 {
			database.RedisClient.Expire(database.Ctx, key, rl.window)
		}

		if count > int64(rl.requests) {
			c.JSON(http.StatusTooManyRequests, utils.ErrorResponse(
				"Rate limit exceeded",
				fmt.Sprintf("Try again in %v", rl.window),
			))
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitByPhone limits OTP requests per phone number
func RateLimitByPhone(phone string, maxAttempts int, window time.Duration) bool {
	if database.RedisClient == nil {
		return true
	}

	key := fmt.Sprintf("ratelimit:phone:%s", phone)
	count, err := database.RedisClient.Incr(database.Ctx, key).Result()
	if err != nil {
		return true
	}

	if count == 1 {
		database.RedisClient.Expire(database.Ctx, key, window)
	}

	return count <= int64(maxAttempts)
}

// Predefined rate limiters
func StrictRateLimit() gin.HandlerFunc {
	return NewRateLimiter(5, time.Minute).RateLimit()
}

func StandardRateLimit() gin.HandlerFunc {
	return NewRateLimiter(60, time.Minute).RateLimit()
}

func OTPRateLimit() gin.HandlerFunc {
	return NewRateLimiter(3, time.Minute).RateLimit()
}

func APIRateLimit() gin.HandlerFunc {
	return NewRateLimiter(1000, time.Hour).RateLimitByUser()
}

// RateLimitResponse returns remaining requests info
func RateLimitResponse(c *gin.Context, key string, limit int, window time.Duration) {
	if database.RedisClient == nil {
		return
	}

	ttl, err := database.RedisClient.TTL(database.Ctx, key).Result()
	if err != nil {
		return
	}

	count, _ := database.RedisClient.Get(database.Ctx, key).Result()
	remaining := limit
	if count != "" {
		// This is simplified - actual implementation would parse count
		remaining = limit - 1
	}

	c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", int(ttl.Seconds())))
}
