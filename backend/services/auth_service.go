package services

import (
	"errors"
	"log"
	"strings"
	"time"

	"rapido-backend/config"
	"rapido-backend/database"
	"rapido-backend/models"
	"rapido-backend/utils"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	DB *gorm.DB
}

func NewAuthService() *AuthService {
	return &AuthService{DB: database.DB}
}

// RequestOTP creates a new OTP for the given phone (using Redis)
func (s *AuthService) RequestOTP(phone string) (*models.OTP, error) {
	// Check if recent OTP exists in Redis
	otpService := NewOTPService()
	if otpService.IsOTPValid(phone, "login") {
		return nil, errors.New("OTP already sent, please wait before requesting again")
	}

	// Generate and store OTP in Redis
	code, err := otpService.GenerateAndStoreOTP(phone, "login")
	if err != nil {
		return nil, err
	}

	// Create minimal record for compatibility (code is hashed, not plain)
	otp := &models.OTP{
		Phone:     phone,
		Code:      "[HASHED-IN-REDIS]", // Never store plain OTP
		Purpose:   "login",
		ExpiresAt: time.Now().Add(time.Duration(config.Get().App.OTPExpiryMin) * time.Minute),
	}

	// Send OTP via SMS
	smsService := NewSMSService()
	if err := smsService.SendOTP(phone, code); err != nil {
		if !isDevMode() {
			otpService.ClearOTP(phone, "login")
			return nil, err
		}
		log.Printf("[AUTH] Failed to send SMS OTP to %s in development: %v", phone, err)
	}

	return otp, nil
}

// VerifyOTP validates OTP code from Redis
func (s *AuthService) VerifyOTP(phone, code string) (*models.OTP, error) {
	otpService := NewOTPService()

	// Verify from Redis (handles rate limiting internally)
	if err := otpService.VerifyOTP(phone, code, "login"); err != nil {
		return nil, err
	}

	// Return minimal OTP record for compatibility
	otp := &models.OTP{
		Phone:   phone,
		Code:    "[VERIFIED]",
		Purpose: "login",
	}

	return otp, nil
}

// LoginOrRegister creates a new user or returns existing one after OTP verification.
// Phone is the primary identity for OTP login; email is used to create or complete the profile.
// userType can be: rider, driver, admin (defaults to rider if not specified)
func (s *AuthService) LoginOrRegister(phone, email, name, userType string) (*models.User, string, string, error) {
	phone = strings.TrimSpace(phone)
	email = strings.ToLower(strings.TrimSpace(email))
	if phone == "" || email == "" {
		return nil, "", "", errors.New("phone and email are required")
	}

	var user models.User

	// Check if user exists
	err := s.DB.Where("phone = ?", phone).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Ensure email is not already linked to a different phone/user.
			var existingByEmail models.User
			if emailErr := s.DB.Where("LOWER(email) = LOWER(?)", email).First(&existingByEmail).Error; emailErr == nil {
				return nil, "", "", errors.New("email already in use by another account")
			}

			// Default to rider if no userType specified
			if userType == "" {
				userType = "rider"
			}
			// Create new user
			user = models.User{
				Phone:    phone,
				Email:    email,
				Name:     name,
				Role:     userType,
				IsActive: true, // Automatically activate new users
			}
			if err := s.DB.Create(&user).Error; err != nil {
				return nil, "", "", err
			}
			queueUserSync("user.upserted", &user)
		} else {
			return nil, "", "", err
		}
	} else {
		updates := make(map[string]interface{})

		if strings.TrimSpace(user.Email) == "" {
			var existingByEmail models.User
			if emailErr := s.DB.Where("LOWER(email) = LOWER(?)", email).First(&existingByEmail).Error; emailErr == nil && existingByEmail.ID != user.ID {
				return nil, "", "", errors.New("email already in use by another account")
			}
			updates["email"] = email
		} else if !strings.EqualFold(strings.TrimSpace(user.Email), email) {
			// Existing accounts can still log in with OTP using the phone number.
			// Keep the stored email unchanged unless it is currently empty.
		}

		// Update name if provided and different
		if name != "" && name != user.Name {
			updates["name"] = name
		}

		// Automatically activate user on login (unless admin deactivated)
		if !user.IsActive {
			updates["is_active"] = true
		}

		if len(updates) > 0 {
			if err := s.DB.Model(&user).Updates(updates).Error; err != nil {
				return nil, "", "", err
			}

			if updatedEmail, ok := updates["email"].(string); ok {
				user.Email = updatedEmail
			}
			if updatedName, ok := updates["name"].(string); ok {
				user.Name = updatedName
			}
			if _, hasActive := updates["is_active"]; hasActive {
				user.IsActive = true
			}
			queueUserSync("user.updated", &user)
		}
	}

	// Generate tokens
	accessToken, _, err := utils.GenerateAccessToken(user.ID.String(), user.Phone, user.Email, user.Role)
	if err != nil {
		return nil, "", "", err
	}

	refreshToken, expiry, err := utils.GenerateRefreshToken(user.ID.String())
	if err != nil {
		return nil, "", "", err
	}

	// Store refresh token
	refresh := &models.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: expiry,
	}
	if err := s.DB.Create(refresh).Error; err != nil {
		return nil, "", "", err
	}

	queueUserSync("user.upserted", &user)

	return &user, accessToken, refreshToken, nil
}

// RefreshToken validates refresh token and issues new access token
func (s *AuthService) RefreshToken(refreshToken string) (*models.User, string, error) {
	// Validate token
	claims, err := utils.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, "", errors.New("invalid refresh token")
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, "", err
	}

	// Check if token exists in DB and not revoked
	var storedToken models.RefreshToken
	if err := s.DB.Where("token = ? AND user_id = ? AND revoked_at IS NULL AND expires_at > ?",
		refreshToken, userID, time.Now()).First(&storedToken).Error; err != nil {
		return nil, "", errors.New("refresh token revoked or expired")
	}

	// Get user
	var user models.User
	if err := s.DB.First(&user, userID).Error; err != nil {
		return nil, "", err
	}

	// Generate new access token
	accessToken, _, err := utils.GenerateAccessToken(user.ID.String(), user.Phone, user.Email, user.Role)
	if err != nil {
		return nil, "", err
	}

	return &user, accessToken, nil
}

// Logout revokes refresh token
func (s *AuthService) Logout(userID uuid.UUID, refreshToken string) error {
	now := time.Now()
	return s.DB.Model(&models.RefreshToken{}).
		Where("user_id = ? AND token = ?", userID, refreshToken).
		Update("revoked_at", now).Error
}

// LogoutAll revokes all refresh tokens for user
func (s *AuthService) LogoutAll(userID uuid.UUID) error {
	now := time.Now()
	return s.DB.Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}

// GetUserByID gets user by ID
func (s *AuthService) GetUserByID(userID uuid.UUID) (*models.User, error) {
	var user models.User
	if err := s.DB.Preload("EmergencyContacts").First(&user, userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUser updates user profile
func (s *AuthService) UpdateUser(userID uuid.UUID, updates map[string]interface{}) (*models.User, error) {
	var user models.User
	if err := s.DB.First(&user, userID).Error; err != nil {
		return nil, err
	}

	// Prevent admin email/password from being changed via API
	// Admin credentials can only be changed via .env file
	if user.Role == "admin" {
		if _, ok := updates["email"]; ok {
			return nil, errors.New("admin email cannot be changed via API - update .env file instead")
		}
		if _, ok := updates["password_hash"]; ok {
			return nil, errors.New("admin password cannot be changed via API - update .env file instead")
		}
	}

	// Check if email is being updated and if it's already taken
	if newEmail, ok := updates["email"].(string); ok && newEmail != "" {
		newEmail = strings.ToLower(strings.TrimSpace(newEmail))
		var existingUser models.User
		if err := s.DB.Where("LOWER(email) = LOWER(?) AND id != ?", newEmail, userID).First(&existingUser).Error; err == nil {
			return nil, errors.New("email already in use by another account")
		}
	}

	if err := s.DB.Model(&user).Updates(updates).Error; err != nil {
		// Check for unique constraint violation
		errStr := err.Error()
		if strings.Contains(errStr, "unique constraint") && strings.Contains(errStr, "email") {
			return nil, errors.New("email already in use by another account")
		}
		return nil, err
	}

	queueUserSync("user.updated", &user)

	return &user, nil
}

// AddEmergencyContact adds emergency contact
func (s *AuthService) AddEmergencyContact(userID uuid.UUID, contact *models.EmergencyContact) (*models.EmergencyContact, error) {
	contact.UserID = userID
	if err := s.DB.Create(contact).Error; err != nil {
		return nil, err
	}
	return contact, nil
}

// RemoveEmergencyContact removes emergency contact
func (s *AuthService) RemoveEmergencyContact(userID, contactID uuid.UUID) error {
	return s.DB.Where("id = ? AND user_id = ?", contactID, userID).Delete(&models.EmergencyContact{}).Error
}

// GoogleLogin handles Google OAuth login
func (s *AuthService) GoogleLogin(googleID, email, phone, name, profileImage string) (*models.User, string, string, error) {
	googleID = strings.TrimSpace(googleID)
	email = strings.ToLower(strings.TrimSpace(email))
	phone = strings.TrimSpace(phone)

	if googleID == "" || email == "" || phone == "" {
		return nil, "", "", errors.New("google_id, email, and phone are required")
	}
	if !utils.IsValidEmail(email) {
		return nil, "", "", errors.New("invalid email")
	}
	if !utils.IsValidPhone(phone) {
		return nil, "", "", errors.New("invalid phone")
	}

	var user models.User

	// Try to find by Google ID
	err := s.DB.Where("google_id = ?", googleID).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Try to find by email
			err = s.DB.Where("LOWER(email) = LOWER(?)", email).First(&user).Error
			if err == nil {
				// Link Google account to existing user and keep phone/email consistent.
				if strings.TrimSpace(user.Phone) == "" {
					var existingByPhone models.User
					if phoneErr := s.DB.Where("phone = ?", phone).First(&existingByPhone).Error; phoneErr == nil && existingByPhone.ID != user.ID {
						return nil, "", "", errors.New("phone already linked to another account")
					}
					user.Phone = phone
				} else if user.Phone != phone {
					return nil, "", "", errors.New("email already linked to a different phone")
				}

				user.GoogleID = googleID
				if user.Name == "" {
					user.Name = name
				}
				if user.ProfileImage == "" {
					user.ProfileImage = profileImage
				}
				if err := s.DB.Save(&user).Error; err != nil {
					return nil, "", "", err
				}
				queueUserSync("user.updated", &user)
			} else {
				// Create new user
				var existingByPhone models.User
				if phoneErr := s.DB.Where("phone = ?", phone).First(&existingByPhone).Error; phoneErr == nil {
					return nil, "", "", errors.New("phone already in use by another account")
				}

				user = models.User{
					Email:         email,
					Phone:         phone,
					Name:          name,
					ProfileImage:  profileImage,
					GoogleID:      googleID,
					Provider:      "google",
					EmailVerified: true,
					Role:          "rider",
				}
				if err := s.DB.Create(&user).Error; err != nil {
					return nil, "", "", err
				}
				queueUserSync("user.upserted", &user)
			}
		} else {
			return nil, "", "", err
		}
	} else {
		if strings.TrimSpace(user.Email) == "" {
			user.Email = email
		} else if !strings.EqualFold(user.Email, email) {
			return nil, "", "", errors.New("google account email mismatch")
		}

		if strings.TrimSpace(user.Phone) == "" {
			var existingByPhone models.User
			if phoneErr := s.DB.Where("phone = ?", phone).First(&existingByPhone).Error; phoneErr == nil && existingByPhone.ID != user.ID {
				return nil, "", "", errors.New("phone already linked to another account")
			}
			user.Phone = phone
		} else if user.Phone != phone {
			return nil, "", "", errors.New("google account phone mismatch")
		}

		if strings.TrimSpace(user.Name) == "" && name != "" {
			user.Name = name
		}
		if strings.TrimSpace(user.ProfileImage) == "" && profileImage != "" {
			user.ProfileImage = profileImage
		}
		if err := s.DB.Save(&user).Error; err != nil {
			return nil, "", "", err
		}
	}

	// Generate tokens
	accessToken, _, err := utils.GenerateAccessToken(user.ID.String(), user.Phone, user.Email, user.Role)
	if err != nil {
		return nil, "", "", err
	}

	refreshToken, expiry, err := utils.GenerateRefreshToken(user.ID.String())
	if err != nil {
		return nil, "", "", err
	}

	refresh := &models.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: expiry,
	}
	if err := s.DB.Create(refresh).Error; err != nil {
		return nil, "", "", err
	}

	return &user, accessToken, refreshToken, nil
}

func queueUserSync(event string, user *models.User) {
	if user == nil {
		return
	}

	QueueCRMEvent(event, "user", user.ID.String(), map[string]interface{}{
		"id":             user.ID.String(),
		"name":           user.Name,
		"email":          user.Email,
		"phone":          user.Phone,
		"role":           user.Role,
		"provider":       user.Provider,
		"provider_id":    user.ProviderID,
		"email_verified": user.EmailVerified,
	})
}

// HashPassword hashes a plain text password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// VerifyPassword compares a plain text password with a hashed password
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// SetPassword sets a new password for a user (first-time setup or password reset)
func (s *AuthService) SetPassword(userID uuid.UUID, password string) error {
	if !utils.IsValidPassword(password) {
		return errors.New("password must be at least 6 characters")
	}

	hashedPassword, err := HashPassword(password)
	if err != nil {
		return errors.New("failed to hash password")
	}

	if err := s.DB.Model(&models.User{}).Where("id = ?", userID).Update("password_hash", hashedPassword).Error; err != nil {
		return err
	}

	log.Printf("[Auth] Password set for user %s", userID)
	return nil
}

// ChangePassword changes password for an existing user (requires old password)
func (s *AuthService) ChangePassword(userID uuid.UUID, oldPassword, newPassword string) error {
	if !utils.IsValidPassword(newPassword) {
		return errors.New("new password must be at least 6 characters")
	}

	var user models.User
	if err := s.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return errors.New("user not found")
	}

	// Verify old password
	if user.PasswordHash == "" {
		return errors.New("no password set - use forgot password to set one")
	}

	if !VerifyPassword(oldPassword, user.PasswordHash) {
		return errors.New("incorrect old password")
	}

	hashedPassword, err := HashPassword(newPassword)
	if err != nil {
		return errors.New("failed to hash password")
	}

	if err := s.DB.Model(&user).Update("password_hash", hashedPassword).Error; err != nil {
		return err
	}

	log.Printf("[Auth] Password changed for user %s", userID)
	return nil
}

// LoginWithPassword authenticates user with email/phone + password
// Returns user, accessToken, refreshToken, requiresPasswordSetup, error
func (s *AuthService) LoginWithPassword(identifier, password string) (*models.User, string, string, bool, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || password == "" {
		return nil, "", "", false, errors.New("email/phone and password are required")
	}

	var user models.User

	// Try to find user by email or phone
	err := s.DB.Where("email = ?", strings.ToLower(identifier)).First(&user).Error
	if err != nil {
		err = s.DB.Where("phone = ?", identifier).First(&user).Error
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[Auth] Login failed: user not found for identifier: %s", identifier)
			return nil, "", "", false, errors.New("invalid credentials")
		}
		return nil, "", "", false, err
	}

	log.Printf("[Auth] User found: %s (ID: %s), Role: %s, HasPassword: %v", user.Email, user.ID, user.Role, user.PasswordHash != "")

	// Check if password is set
	if user.PasswordHash == "" {
		// User exists but no password - requires OTP verification first
		log.Printf("[Auth] Login failed: user %s has no password set", user.ID)
		return &user, "", "", true, errors.New("password not set - please login with OTP first")
	}

	// Verify password
	if !VerifyPassword(password, user.PasswordHash) {
		log.Printf("[Auth] Login failed: password mismatch for user %s", user.ID)
		return nil, "", "", false, errors.New("invalid credentials")
	}

	log.Printf("[Auth] Password login successful for user: %s", user.ID)

	// Generate tokens
	accessToken, _, err := utils.GenerateAccessToken(user.ID.String(), user.Phone, user.Email, user.Role)
	if err != nil {
		return nil, "", "", false, err
	}

	refreshToken, _, err := utils.GenerateRefreshToken(user.ID.String())
	if err != nil {
		return nil, "", "", false, err
	}

	// Store refresh token
	refreshTokenModel := &models.RefreshToken{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(config.Get().JWT.RefreshExpiry),
	}
	if err := s.DB.Create(refreshTokenModel).Error; err != nil {
		return nil, "", "", false, err
	}

	return &user, accessToken, refreshToken, false, nil
}

// HasPasswordSet checks if a user has set their password
func (s *AuthService) HasPasswordSet(userID uuid.UUID) (bool, error) {
	var user models.User
	if err := s.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return false, err
	}
	return user.PasswordHash != "", nil
}
