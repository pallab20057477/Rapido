package middleware

import (
	"context"
	"net/http"
	"time"

	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// OTPAttemptPrefix Redis key prefix
	OTPAttemptPrefix = "otp:attempts:"
	// OTPBlockedPrefix Redis key prefix
	OTPBlockedPrefix = "otp:blocked:"
	// MaxOTPAttempts maximum allowed attempts
	MaxOTPAttempts = 5
	// OTPBlockDuration how long to block after max attempts
	OTPBlockDuration = 15 * time.Minute
	// OTPAttemptWindow time window for counting attempts
	OTPAttemptWindow = 1 * time.Hour
)

// OTPRateLimitMiddleware enforces OTP attempt limits
func OTPRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		phone := extractPhoneFromRequest(c)
		if phone == "" {
			c.Next()
			return
		}

		// Check if blocked
		if isOTPBlocked(phone) {
			utils.Warn("OTP attempt from blocked phone", zap.String("phone", maskPhone(phone)))
			c.JSON(http.StatusTooManyRequests, utils.ErrorResponse(
				"Too many attempts. Please try again after 15 minutes.",
				"otp_blocked",
			))
			c.Abort()
			return
		}

		c.Next()

		// If response was error, increment attempt counter
		if c.Writer.Status() >= 400 {
			incrementOTPAttempts(phone)
		}
	}
}

// OTPVerifyRateLimitMiddleware for verify endpoint (stricter)
func OTPVerifyRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		phone := extractPhoneFromRequest(c)
		if phone == "" {
			c.Next()
			return
		}

		// Check if blocked
		if isOTPBlocked(phone) {
			utils.Warn("OTP verify attempt from blocked phone", zap.String("phone", maskPhone(phone)))
			c.JSON(http.StatusTooManyRequests, utils.ErrorResponse(
				"Too many failed attempts. Please request new OTP after 15 minutes.",
				"otp_blocked",
			))
			c.Abort()
			return
		}

		c.Next()

		// Increment on any request to verify endpoint
		incrementOTPAttempts(phone)
	}
}

// isOTPBlocked checks if phone is blocked
func isOTPBlocked(phone string) bool {
	if database.RedisClient == nil {
		return false
	}

	ctx := context.Background()
	key := OTPBlockedPrefix + phone

	exists, err := database.RedisClient.Exists(ctx, key).Result()
	if err != nil {
		utils.Error("failed to check OTP block status", zap.Error(err))
		return false
	}

	return exists > 0
}

// incrementOTPAttempts increments attempt counter and blocks if exceeded
func incrementOTPAttempts(phone string) {
	if database.RedisClient == nil {
		return
	}

	ctx := context.Background()
	key := OTPAttemptPrefix + phone

	// Increment counter
	pipe := database.RedisClient.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, OTPAttemptWindow)

	results, err := pipe.Exec(ctx)
	if err != nil {
		utils.Error("failed to increment OTP attempts", utils.ErrorLogFields(err, "redis_error", "")...)
		return
	}

	// Check if should block
	count := results[0].(*redis.IntCmd).Val()
	if count >= MaxOTPAttempts {
		blockOTP(phone)
		utils.Warn("phone blocked due to OTP abuse",
			zap.String("phone", maskPhone(phone)),
			zap.Int64("attempts", count),
		)
	}
}

// blockOTP blocks a phone number
func blockOTP(phone string) {
	if database.RedisClient == nil {
		return
	}

	ctx := context.Background()
	key := OTPBlockedPrefix + phone

	database.RedisClient.Set(ctx, key, "1", OTPBlockDuration)
}

// ResetOTPAttempts resets attempt counter (called on successful verification)
func ResetOTPAttempts(phone string) {
	if database.RedisClient == nil {
		return
	}

	ctx := context.Background()
	key := OTPAttemptPrefix + phone

	database.RedisClient.Del(ctx, key)
}

// extractPhoneFromRequest extracts phone from request body
func extractPhoneFromRequest(c *gin.Context) string {
	var body struct {
		Phone string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&body); err == nil && body.Phone != "" {
		return body.Phone
	}
	return c.Query("phone")
}

// maskPhone masks phone number for logging
func maskPhone(phone string) string {
	if len(phone) < 8 {
		return "***"
	}
	return phone[:3] + "****" + phone[len(phone)-2:]
}

// GetOTPAttemptStatus returns current attempt status for monitoring
func GetOTPAttemptStatus(phone string) (attempts int64, blocked bool, remaining int64) {
	if database.RedisClient == nil {
		return 0, false, MaxOTPAttempts
	}

	ctx := context.Background()

	// Check blocked status
	blockKey := OTPBlockedPrefix + phone
	blockedCount, _ := database.RedisClient.Exists(ctx, blockKey).Result()
	if blockedCount > 0 {
		return MaxOTPAttempts, true, 0
	}

	// Get attempt count
	attemptKey := OTPAttemptPrefix + phone
	attempts, _ = database.RedisClient.Get(ctx, attemptKey).Int64()

	remaining = MaxOTPAttempts - attempts
	if remaining < 0 {
		remaining = 0
	}

	return attempts, false, remaining
}
