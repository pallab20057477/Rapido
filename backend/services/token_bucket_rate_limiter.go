package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"rapido-backend/database"
)

// TokenBucketRateLimiter implements production-grade rate limiting with:
// - Token bucket algorithm (smooth rate limiting)
// - Redis-backed distributed state
// - Per-endpoint and per-user tiered limits
// - Burst capacity for traffic spikes
// - X-RateLimit headers for client feedback
type TokenBucketRateLimiter struct {
	redis *redis.Client
}

// RateLimitTier defines limits for different endpoint categories
type RateLimitTier struct {
	Name       string
	Capacity   int     // Maximum tokens in bucket
	RefillRate float64 // Tokens per second
	BurstSize  int     // Maximum burst
}

// Default rate limit tiers (FAANG-level configuration)
var (
	// CriticalTier: Ride requests, payments (very strict)
	CriticalTier = RateLimitTier{
		Name:       "critical",
		Capacity:   10,
		RefillRate: 0.167, // 10 per minute = 1 per 6 seconds
		BurstSize:  3,
	}

	// StandardTier: Status checks, profile (moderate)
	StandardTier = RateLimitTier{
		Name:       "standard",
		Capacity:   100,
		RefillRate: 1.67, // 100 per minute
		BurstSize:  20,
	}

	// HeavyTier: List operations, search (lower)
	HeavyTier = RateLimitTier{
		Name:       "heavy",
		Capacity:   30,
		RefillRate: 0.5, // 30 per minute
		BurstSize:  10,
	}

	// PublicTier: Login, OTP (strict to prevent abuse)
	PublicTier = RateLimitTier{
		Name:       "public",
		Capacity:   5,
		RefillRate: 0.083, // 5 per minute
		BurstSize:  2,
	}
)

// NewTokenBucketRateLimiter creates a rate limiter instance
func NewTokenBucketRateLimiter() *TokenBucketRateLimiter {
	return &TokenBucketRateLimiter{
		redis: database.RedisClient,
	}
}

// Middleware returns Gin middleware for rate limiting
func (rl *TokenBucketRateLimiter) Middleware(tier RateLimitTier) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user identifier (userID for authenticated, IP for anonymous)
		userID := rl.getUserIdentifier(c)
		endpoint := c.FullPath()

		// Check rate limit
		allowed, remaining, resetAt, err := rl.checkLimit(userID, endpoint, tier)
		if err != nil {
			// On error, allow request but log
			c.Next()
			return
		}

		// Add rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(tier.Capacity))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt, 10))
		c.Header("X-RateLimit-Tier", tier.Name)

		if !allowed {
			c.Header("Retry-After", strconv.Itoa(int(resetAt-time.Now().Unix())))
			c.JSON(429, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": resetAt - time.Now().Unix(),
				"tier":        tier.Name,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// checkLimit evaluates if request is allowed using token bucket algorithm
func (rl *TokenBucketRateLimiter) checkLimit(userID, endpoint string, tier RateLimitTier) (bool, int, int64, error) {
	ctx := context.Background()
	key := fmt.Sprintf("ratelimit:%s:%s:%s", tier.Name, userID, endpoint)

	// Lua script for atomic token bucket operation
	script := `
		local key = KEYS[1]
		local capacity = tonumber(ARGV[1])
		local refill_rate = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		
		-- Get current bucket state
		local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
		local tokens = tonumber(bucket[1])
		local last_refill = tonumber(bucket[2])
		
		-- Initialize if not exists
		if tokens == nil then
			tokens = capacity
			last_refill = now
		end
		
		-- Calculate tokens to add based on time passed
		local time_passed = math.max(0, now - last_refill)
		local tokens_to_add = time_passed * refill_rate
		tokens = math.min(capacity, tokens + tokens_to_add)
		
		-- Check if we can consume a token
		local allowed = 0
		if tokens >= 1 then
			tokens = tokens - 1
			allowed = 1
		end
		
		-- Update bucket state
		redis.call('HMSET', key, 'tokens', tokens, 'last_refill', now)
		redis.call('EXPIRE', key, 3600) -- 1 hour TTL
		
		-- Calculate reset time (when 1 token will be available)
		local reset_at = now
		if tokens < capacity then
			reset_at = now + math.ceil((1 - tokens) / refill_rate)
		end
		
		return {allowed, math.floor(tokens), reset_at}
	`

	now := time.Now().Unix()
	result, err := rl.redis.Eval(ctx, script, []string{key},
		tier.Capacity, tier.RefillRate, now).Result()

	if err != nil {
		return true, tier.Capacity, now, err // Fail open
	}

	values := result.([]interface{})
	allowed := values[0].(int64) == 1
	remaining := int(values[1].(int64))
	resetAt := values[2].(int64)

	return allowed, remaining, resetAt, nil
}

// IsAllowed checks if a specific action is allowed (for non-HTTP use)
func (rl *TokenBucketRateLimiter) IsAllowed(userID, action string, tier RateLimitTier) bool {
	allowed, _, _, _ := rl.checkLimit(userID, action, tier)
	return allowed
}

// Reset resets the rate limit for a user (e.g., after payment)
func (rl *TokenBucketRateLimiter) Reset(userID, endpoint string, tier RateLimitTier) {
	ctx := context.Background()
	key := fmt.Sprintf("ratelimit:%s:%s:%s", tier.Name, userID, endpoint)
	rl.redis.Del(ctx, key)
}

// GetStatus returns current rate limit status for a user
func (rl *TokenBucketRateLimiter) GetStatus(userID, endpoint string, tier RateLimitTier) (remaining int, resetAt int64, err error) {
	ctx := context.Background()
	key := fmt.Sprintf("ratelimit:%s:%s:%s", tier.Name, userID, endpoint)

	bucket, err := rl.redis.HMGet(ctx, key, "tokens", "last_refill").Result()
	if err != nil {
		return tier.Capacity, time.Now().Unix(), err
	}

	tokens := tier.Capacity
	if bucket[0] != nil {
		tokens, _ = strconv.Atoi(bucket[0].(string))
	}

	return tokens, time.Now().Add(time.Minute).Unix(), nil
}

// getUserIdentifier extracts user identifier from context
func (rl *TokenBucketRateLimiter) getUserIdentifier(c *gin.Context) string {
	// Try to get authenticated user ID
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(string); ok {
			return id
		}
	}

	// Fall back to IP address
	clientIP := c.ClientIP()
	return "ip:" + clientIP
}

// Global instance
var TokenBucketRateLimiterInstance *TokenBucketRateLimiter

// InitTokenBucketRateLimiter initializes the global rate limiter
func InitTokenBucketRateLimiter() {
	TokenBucketRateLimiterInstance = NewTokenBucketRateLimiter()
}
