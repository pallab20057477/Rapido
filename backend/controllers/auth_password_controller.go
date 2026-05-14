package controllers

import (
	"net/http"

	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PasswordLoginRequest request body for password login
type PasswordLoginRequest struct {
	Identifier string `json:"identifier" binding:"required"` // email or phone
	Password   string `json:"password" binding:"required"`
}

// PasswordLogin authenticates user with email/phone + password
func (c *AuthController) PasswordLogin(ctx *gin.Context) {
	var req PasswordLoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	user, accessToken, refreshToken, requiresPasswordSetup, err := c.Service.LoginWithPassword(req.Identifier, req.Password)

	if err != nil {
		if requiresPasswordSetup {
			// User exists but password not set
			ctx.JSON(http.StatusForbidden, utils.ErrorResponse("Password not set", "Please login with OTP first and set your password"))
			return
		}
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid credentials", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Login successful", map[string]interface{}{
		"user":          user,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
	}))
}

// SetPasswordRequest request body for setting password
type SetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=6"`
}

// SetPassword sets password for first-time user after OTP verification
func (c *AuthController) SetPassword(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	userRole := ctx.GetString("userRole")
	uid, err := uuid.Parse(userID)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid user", ""))
		return
	}

	// Prevent admin from setting password via API - only through .env
	if userRole == "admin" {
		ctx.JSON(http.StatusForbidden, utils.ErrorResponse("Forbidden", "admin password can only be set via .env file"))
		return
	}

	var req SetPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Validate password strength
	if !utils.IsValidPassword(req.Password) {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid password", "Password must be at least 6 characters"))
		return
	}

	if err := c.Service.SetPassword(uid, req.Password); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to set password", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Password set successfully", map[string]interface{}{
		"message":        "You can now login with your email/phone and password",
		"login_endpoint": "/api/v1/auth/login",
	}))
}

// ChangePasswordRequest request body for changing password
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ChangePassword changes password for logged-in user
func (c *AuthController) ChangePassword(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	userRole := ctx.GetString("userRole")
	uid, err := uuid.Parse(userID)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid user", ""))
		return
	}

	// Prevent admin from changing password via API - only through .env
	if userRole == "admin" {
		ctx.JSON(http.StatusForbidden, utils.ErrorResponse("Forbidden", "admin password can only be changed via .env file"))
		return
	}

	var req ChangePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Validate new password
	if !utils.IsValidPassword(req.NewPassword) {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid password", "Password must be at least 6 characters"))
		return
	}

	if err := c.Service.ChangePassword(uid, req.OldPassword, req.NewPassword); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Failed to change password", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Password changed successfully", nil))
}

// HasPassword checks if user has set password
func (c *AuthController) HasPassword(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, err := uuid.Parse(userID)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid user", ""))
		return
	}

	hasPassword, err := c.Service.HasPasswordSet(uid)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to check password status", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Password status", map[string]interface{}{
		"has_password": hasPassword,
	}))
}
