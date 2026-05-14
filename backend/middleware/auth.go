package middleware

import (
	"net/http"
	"strings"

	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
)

func setUserContext(c *gin.Context, claims *utils.TokenClaims) {
	c.Set("userID", claims.UserID)
	c.Set("user_id", claims.UserID)
	c.Set("userPhone", claims.Phone)
	c.Set("userEmail", claims.Email)
	c.Set("userRole", claims.Role)
	c.Set("user_type", claims.Role)
}

// AuthMiddleware validates JWT token and sets user context
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", "missing authorization header"))
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", "invalid authorization format"))
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := utils.ValidateAccessToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, utils.ErrorResponse("Unauthorized", err.Error()))
			c.Abort()
			return
		}

		// Set user info in context using both canonical and legacy keys.
		setUserContext(c, claims)

		c.Next()
	}
}

// OptionalAuthMiddleware allows requests without auth but sets user context if present
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Next()
			return
		}

		tokenString := parts[1]
		claims, err := utils.ValidateAccessToken(tokenString)
		if err != nil {
			c.Next()
			return
		}

		setUserContext(c, claims)

		c.Next()
	}
}

// RoleMiddleware checks if user has required role
func RoleMiddleware(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("userRole")
		if !exists {
			c.JSON(http.StatusForbidden, utils.ErrorResponse("Forbidden", "role not found in context"))
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok {
			c.JSON(http.StatusForbidden, utils.ErrorResponse("Forbidden", "invalid role type"))
			c.Abort()
			return
		}

		for _, role := range roles {
			if role == roleStr {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, utils.ErrorResponse("Forbidden", "insufficient permissions"))
		c.Abort()
	}
}

// AdminMiddleware ensures user is an admin
func AdminMiddleware() gin.HandlerFunc {
	return RoleMiddleware("admin", "super_admin")
}

// DriverMiddleware ensures user is a driver
func DriverMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("userRole")
		if !exists {
			c.JSON(http.StatusForbidden, utils.ErrorResponse("Forbidden", "not a driver"))
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok || roleStr != "driver" {
			c.JSON(http.StatusForbidden, utils.ErrorResponse("Forbidden", "driver access required"))
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserID extracts user ID from context
func GetUserID(c *gin.Context) string {
	userID, exists := c.Get("userID")
	if !exists {
		return ""
	}
	return userID.(string)
}

// GetUserRole extracts user role from context
func GetUserRole(c *gin.Context) string {
	userRole, exists := c.Get("userRole")
	if !exists {
		return ""
	}
	return userRole.(string)
}
