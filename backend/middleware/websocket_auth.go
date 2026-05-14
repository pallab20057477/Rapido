package middleware

import (
	"net/http"
	"strings"
	"time"

	"rapido-backend/config"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// WebSocketAuthMiddleware validates JWT for WebSocket connections
func WebSocketAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// WebSocket upgrade request must have valid JWT
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" && allowWebSocketQueryToken() {
			// Allow query-token auth only in development to avoid leaking credentials in production URLs.
			authHeader = c.Query("token")
		}

		if authHeader == "" {
			utils.Warn("WebSocket connection attempt without auth header",
				zap.String("ip", c.ClientIP()))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			return
		}

		// Extract token
		tokenString := authHeader
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			tokenString = parts[1]
		}

		// Validate token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(config.AppConfigInstance.JWT.Secret), nil
		})

		if err != nil || !token.Valid {
			utils.Warn("Invalid WebSocket JWT",
				zap.String("ip", c.ClientIP()),
				zap.Error(err))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			return
		}

		// Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token claims",
			})
			return
		}

		// Validate required claims
		userID, ok := claims["user_id"].(string)
		if !ok || userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid user_id in token",
			})
			return
		}

		userType, ok := claims["role"].(string)
		if !ok || userType == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid role in token",
			})
			return
		}

		// Check token expiration
		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().Unix() > int64(exp) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Token expired",
				})
				return
			}
		}

		// Check if token is blacklisted (user logged out)
		if IsTokenBlacklisted(tokenString) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Token revoked",
			})
			return
		}

		// Set authenticated user info in context
		c.Set("userID", userID)
		c.Set("user_id", userID)
		c.Set("userRole", userType)
		c.Set("user_type", userType)
		c.Set("token", tokenString)

		// Rate limiting per user
		if !checkWebSocketRateLimit(userID) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many WebSocket connection attempts",
			})
			return
		}

		utils.Info("WebSocket auth successful",
			zap.String("user_id", userID),
			zap.String("user_type", userType),
			zap.String("ip", c.ClientIP()))

		c.Next()
	}
}

func allowWebSocketQueryToken() bool {
	env := strings.ToLower(strings.TrimSpace(config.Get().App.Environment))
	return env == "development" || env == "dev"
}

// WebSocketQueryValidator validates query params match JWT claims
func WebSocketQueryValidator() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user info from JWT (set by auth middleware)
		jwtUserID, exists := c.Get("userID")
		if !exists {
			jwtUserID, exists = c.Get("user_id")
		}
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			return
		}

		jwtUserType, exists := c.Get("userRole")
		if !exists {
			jwtUserType, exists = c.Get("user_type")
		}
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			return
		}

		// Get query params (legacy - will be removed)
		queryUserID := c.Query("user_id")
		queryUserType := c.Query("user_type")

		// If query params provided, validate they match JWT
		if queryUserID != "" && queryUserID != jwtUserID.(string) {
			utils.Warn("WebSocket user_id mismatch",
				zap.String("jwt_user", jwtUserID.(string)),
				zap.String("query_user", queryUserID))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "user_id mismatch",
			})
			return
		}

		if queryUserType != "" && queryUserType != jwtUserType.(string) {
			utils.Warn("WebSocket user_type mismatch",
				zap.String("jwt_type", jwtUserType.(string)),
				zap.String("query_type", queryUserType))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "user_type mismatch",
			})
			return
		}

		// Set the validated user info
		c.Set("validated_user_id", jwtUserID.(string))
		c.Set("validated_user_type", jwtUserType.(string))

		c.Next()
	}
}

// checkWebSocketRateLimit limits connection attempts per user
func checkWebSocketRateLimit(userID string) bool {
	// Rate limiting implemented via Redis - simplified for now
	return true
}
