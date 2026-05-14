package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"rapido-backend/config"
	"rapido-backend/models"
	"rapido-backend/services"
	"rapido-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthController struct {
	Service *services.AuthService
}

func NewAuthController() *AuthController {
	return &AuthController{Service: services.NewAuthService()}
}

// RequestOTPRequest request body for OTP
type RequestOTPRequest struct {
	Phone string `json:"phone" binding:"required"`
}

// RequestOTP sends OTP to phone
func (c *AuthController) RequestOTP(ctx *gin.Context) {
	var req RequestOTPRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	// Sanitize phone
	phone := utils.SanitizePhone(req.Phone)
	if !utils.IsValidPhone(phone) {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid phone number", ""))
		return
	}

	_, err := c.Service.RequestOTP(phone)
	if err != nil {
		ctx.JSON(http.StatusTooManyRequests, utils.SanitizedErrorResponse("Failed to send OTP", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("OTP sent successfully", map[string]interface{}{
		"phone":      utils.MaskPhone(phone),
		"expires_in": 300,
	}))
}

// VerifyOTPRequest request body for OTP verification
type VerifyOTPRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Email    string `json:"email" binding:"required"`
	OTP      string `json:"otp" binding:"required"`
	Name     string `json:"name,omitempty"`
	UserType string `json:"user_type,omitempty"` // rider, driver, admin
}

// VerifyOTP verifies OTP and logs in/creates user
func (c *AuthController) VerifyOTP(ctx *gin.Context) {
	var req VerifyOTPRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	phone := utils.SanitizePhone(req.Phone)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !utils.IsValidEmail(email) {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid email", ""))
		return
	}

	// Verify OTP
	_, err := c.Service.VerifyOTP(phone, req.OTP)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, utils.SanitizedErrorResponse("Invalid OTP", err.Error()))
		return
	}

	// Login or register (default to 'rider' if user_type not specified)
	userType := req.UserType
	if userType == "" {
		userType = "rider"
	}
	user, accessToken, refreshToken, err := c.Service.LoginOrRegister(phone, email, req.Name, userType)
	if err != nil {
		if strings.Contains(err.Error(), "already linked") || strings.Contains(err.Error(), "already in use") || strings.Contains(err.Error(), "required") {
			ctx.JSON(http.StatusBadRequest, utils.SanitizedErrorResponse("Login failed", err.Error()))
			return
		}
		ctx.JSON(http.StatusInternalServerError, utils.SanitizedErrorResponse("Login failed", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Login successful", map[string]interface{}{
		"user":          user,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
	}))
}

// RefreshTokenRequest request body for token refresh
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshToken refreshes access token
func (c *AuthController) RefreshToken(ctx *gin.Context) {
	var req RefreshTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	user, accessToken, err := c.Service.RefreshToken(req.RefreshToken)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid refresh token", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Token refreshed", map[string]interface{}{
		"user":         user,
		"access_token": accessToken,
		"token_type":   "Bearer",
	}))
}

// Logout logs out user
func (c *AuthController) Logout(ctx *gin.Context) {
	userID := ctx.GetString("userID")

	var req RefreshTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	uid, _ := uuid.Parse(userID)
	if err := c.Service.Logout(uid, req.RefreshToken); err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Logout failed", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Logged out successfully", nil))
}

// GetProfile gets user profile
func (c *AuthController) GetProfile(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	user, err := c.Service.GetUserByID(uid)
	if err != nil {
		ctx.JSON(http.StatusNotFound, utils.ErrorResponse("User not found", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Profile retrieved", user))
}

// UpdateProfileRequest request body for profile update
type UpdateProfileRequest struct {
	Name         string `json:"name,omitempty"`
	Email        string `json:"email,omitempty"`
	ProfileImage string `json:"profile_image,omitempty"`
}

// UpdateProfile updates user profile
func (c *AuthController) UpdateProfile(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	var req UpdateProfileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Email != "" {
		if !utils.IsValidEmail(req.Email) {
			ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid email", ""))
			return
		}
		updates["email"] = req.Email
	}
	if req.ProfileImage != "" {
		updates["profile_image"] = req.ProfileImage
	}

	user, err := c.Service.UpdateUser(uid, updates)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Update failed", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Profile updated", user))
}

// AddEmergencyContactRequest request body
type AddEmergencyContactRequest struct {
	Name      string `json:"name" binding:"required"`
	Phone     string `json:"phone" binding:"required"`
	Relation  string `json:"relation,omitempty"`
	IsPrimary bool   `json:"is_primary,omitempty"`
}

// AddEmergencyContact adds emergency contact
func (c *AuthController) AddEmergencyContact(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	var req AddEmergencyContactRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	contact := &models.EmergencyContact{
		Name:         req.Name,
		Phone:        req.Phone,
		Relationship: req.Relation,
		Priority:     1,
	}

	contact, err := c.Service.AddEmergencyContact(uid, contact)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to add contact", err.Error()))
		return
	}

	ctx.JSON(http.StatusCreated, utils.SuccessResponse("Emergency contact added", contact))
}

// RemoveEmergencyContact removes emergency contact
func (c *AuthController) RemoveEmergencyContact(ctx *gin.Context) {
	userID := ctx.GetString("userID")
	uid, _ := uuid.Parse(userID)

	contactID := ctx.Param("id")
	cid, err := uuid.Parse(contactID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid contact ID", ""))
		return
	}

	if err := c.Service.RemoveEmergencyContact(uid, cid); err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Failed to remove contact", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Emergency contact removed", nil))
}

// GoogleLoginRequest request body for Google login
type GoogleLoginRequest struct {
	IDToken string `json:"id_token" binding:"required"`
	Phone   string `json:"phone" binding:"required"`
}

type googleTokenInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Aud           string `json:"aud"`
	Iss           string `json:"iss"`
	EmailVerified string `json:"email_verified"`
}

func verifyGoogleIDToken(idToken string) (*googleTokenInfo, error) {
	resp, err := http.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + url.QueryEscape(idToken))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google token invalid: %s", string(body))
	}

	var info googleTokenInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}

	if info.Sub == "" || info.Email == "" {
		return nil, fmt.Errorf("google token missing required claims")
	}

	if info.EmailVerified != "true" {
		return nil, fmt.Errorf("google email is not verified")
	}

	if info.Iss != "accounts.google.com" && info.Iss != "https://accounts.google.com" {
		return nil, fmt.Errorf("google token issuer mismatch")
	}

	clientID := strings.TrimSpace(config.Get().Google.ClientID)
	if clientID != "" && info.Aud != clientID {
		return nil, fmt.Errorf("google token audience mismatch")
	}

	return &info, nil
}

// GoogleLogin handles Google OAuth login
func (c *AuthController) GoogleLogin(ctx *gin.Context) {
	var req GoogleLoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid request", err.Error()))
		return
	}

	phone := utils.SanitizePhone(req.Phone)
	if !utils.IsValidPhone(phone) {
		ctx.JSON(http.StatusBadRequest, utils.ErrorResponse("Invalid phone number", ""))
		return
	}

	info, err := verifyGoogleIDToken(req.IDToken)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, utils.ErrorResponse("Invalid Google token", err.Error()))
		return
	}

	email := strings.ToLower(strings.TrimSpace(info.Email))

	user, accessToken, refreshToken, err := c.Service.GoogleLogin(info.Sub, email, phone, info.Name, info.Picture)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse("Google login failed", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, utils.SuccessResponse("Login successful", map[string]interface{}{
		"user":          user,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
	}))
}
