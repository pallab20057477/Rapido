package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// TokenBlacklistPrefix for Redis keys
	TokenBlacklistPrefix = "jwt:blacklist:"
	// RefreshTokenPrefix for Redis keys
	RefreshTokenPrefix = "jwt:refresh:"
	// TokenExpiryBuffer prevents using tokens close to expiry
	TokenExpiryBuffer = 5 * time.Minute
)

// TokenBlacklistMiddleware checks if JWT token is blacklisted
func TokenBlacklistMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		// Extract token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Next()
			return
		}

		tokenString := parts[1]

		// Check if blacklisted
		if IsTokenBlacklisted(tokenString) {
			utils.Warn("blacklisted token used", utils.ErrorLogFields(
				fmt.Errorf("token blacklisted"),
				"security_violation",
				c.Request.URL.Path,
			)...)
			c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Token revoked", ""))
			c.Abort()
			return
		}

		c.Next()
	}
}

// IsTokenBlacklisted checks if a token is in the blacklist
func IsTokenBlacklisted(tokenString string) bool {
	if database.RedisClient == nil {
		return false
	}

	ctx := context.Background()
	key := TokenBlacklistPrefix + hashToken(tokenString)

	exists, err := database.RedisClient.Exists(ctx, key).Result()
	if err != nil {
		utils.Error("failed to check token blacklist", zap.Error(err))
		return false
	}

	return exists > 0
}

// BlacklistToken adds a token to the blacklist
func BlacklistToken(tokenString string, expiresAt time.Time) error {
	if database.RedisClient == nil {
		return fmt.Errorf("redis not available")
	}

	ctx := context.Background()
	key := TokenBlacklistPrefix + hashToken(tokenString)

	// Calculate TTL
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ttl = 24 * time.Hour // Default TTL if token already expired
	}

	return database.RedisClient.Set(ctx, key, "1", ttl).Err()
}

// BlacklistUserTokens blacklists all tokens for a user (on password change, security breach)
func BlacklistUserTokens(userID uuid.UUID) error {
	if database.RedisClient == nil {
		return fmt.Errorf("redis not available")
	}

	ctx := context.Background()
	key := fmt.Sprintf("%suser:%s", TokenBlacklistPrefix, userID.String())

	// Set a flag that invalidates all user's tokens
	return database.RedisClient.Set(ctx, key, time.Now().Unix(), 24*time.Hour).Err()
}

// IsUserTokensBlacklisted checks if user's tokens should be invalidated
func IsUserTokensBlacklisted(userID uuid.UUID, tokenIssuedAt time.Time) bool {
	if database.RedisClient == nil {
		return false
	}

	ctx := context.Background()
	key := fmt.Sprintf("%suser:%s", TokenBlacklistPrefix, userID.String())

	blacklistTime, err := database.RedisClient.Get(ctx, key).Int64()
	if err == redis.Nil {
		return false
	}
	if err != nil {
		utils.Error("failed to check user token blacklist", zap.Error(err))
		return false
	}

	// If token was issued before blacklist time, it's invalid
	return tokenIssuedAt.Before(time.Unix(blacklistTime, 0))
}

// StoreRefreshToken stores refresh token with rotation tracking
func StoreRefreshToken(userID uuid.UUID, tokenID string, expiresAt time.Time) error {
	if database.RedisClient == nil {
		return fmt.Errorf("redis not available")
	}

	ctx := context.Background()
	key := RefreshTokenPrefix + userID.String()

	// Store token ID with expiry
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return fmt.Errorf("token already expired")
	}

	// Use Redis set to track all active refresh tokens for user
	return database.RedisClient.Set(ctx, key+":"+tokenID, time.Now().Unix(), ttl).Err()
}

// ValidateRefreshToken checks if refresh token is valid and not revoked
func ValidateRefreshToken(userID uuid.UUID, tokenID string) (bool, error) {
	if database.RedisClient == nil {
		return true, nil // Allow if Redis unavailable (degraded mode)
	}

	ctx := context.Background()
	key := RefreshTokenPrefix + userID.String() + ":" + tokenID

	exists, err := database.RedisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return exists > 0, nil
}

// RevokeRefreshToken revokes a specific refresh token
func RevokeRefreshToken(userID uuid.UUID, tokenID string) error {
	if database.RedisClient == nil {
		return fmt.Errorf("redis not available")
	}

	ctx := context.Background()
	key := RefreshTokenPrefix + userID.String() + ":" + tokenID

	return database.RedisClient.Del(ctx, key).Err()
}

// RevokeAllUserRefreshTokens revokes all refresh tokens for a user
func RevokeAllUserRefreshTokens(userID uuid.UUID) error {
	if database.RedisClient == nil {
		return fmt.Errorf("redis not available")
	}

	ctx := context.Background()
	pattern := RefreshTokenPrefix + userID.String() + ":*"

	// Find and delete all tokens
	iter := database.RedisClient.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		database.RedisClient.Del(ctx, iter.Val())
	}

	return iter.Err()
}

// RotateRefreshToken atomically revokes old token and stores new one
func RotateRefreshToken(userID uuid.UUID, oldTokenID, newTokenID string, newExpiresAt time.Time) error {
	if database.RedisClient == nil {
		return fmt.Errorf("redis not available")
	}

	ctx := context.Background()
	pipe := database.RedisClient.Pipeline()

	// Revoke old token
	oldKey := RefreshTokenPrefix + userID.String() + ":" + oldTokenID
	pipe.Del(ctx, oldKey)

	// Store new token
	newKey := RefreshTokenPrefix + userID.String() + ":" + newTokenID
	ttl := time.Until(newExpiresAt)
	pipe.Set(ctx, newKey, time.Now().Unix(), ttl)

	_, err := pipe.Exec(ctx)
	return err
}

// hashToken creates a short hash of token for Redis key
func hashToken(token string) string {
	// Simple hash - in production use SHA-256
	if len(token) < 16 {
		return token
	}
	return token[:16] + token[len(token)-16:]
}

// ExtractTokenClaims extracts claims without validation (for blacklist check)
func ExtractTokenClaims(tokenString string) (jwt.MapClaims, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid claims")
}
