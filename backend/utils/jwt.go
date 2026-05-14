package utils

import (
	"errors"
	"fmt"
	"time"

	"rapido-backend/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Token claims
type TokenClaims struct {
	UserID   string `json:"user_id"`
	Phone    string `json:"phone"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// GenerateAccessToken creates a new access token
func GenerateAccessToken(userID, phone, email, role string) (string, time.Time, error) {
	cfg := config.Get().JWT
	expiryTime := time.Now().Add(cfg.AccessExpiry)

	claims := TokenClaims{
		UserID:   userID,
		Phone:    phone,
		Email:    email,
		Role:     role,
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiryTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "rapido-backend",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.Secret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiryTime, nil
}

// GenerateRefreshToken creates a new refresh token
func GenerateRefreshToken(userID string) (string, time.Time, error) {
	cfg := config.Get().JWT
	expiryTime := time.Now().Add(cfg.RefreshExpiry)

	claims := TokenClaims{
		UserID:    userID,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiryTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "rapido-backend",
			Subject:   userID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.RefreshSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiryTime, nil
}

// ValidateAccessToken validates and parses an access token
func ValidateAccessToken(tokenString string) (*TokenClaims, error) {
	cfg := config.Get().JWT

	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token expired")
		}
		return nil, err
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		if claims.TokenType != "access" {
			return nil, errors.New("invalid token type")
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// ValidateRefreshToken validates and parses a refresh token
func ValidateRefreshToken(tokenString string) (*TokenClaims, error) {
	cfg := config.Get().JWT

	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.RefreshSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token expired")
		}
		return nil, err
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		if claims.TokenType != "refresh" {
			return nil, errors.New("invalid token type")
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// GenerateRideOTP generates a 4-digit OTP for ride verification
func GenerateRideOTP() string {
	// Generate random 4 digit number
	return fmt.Sprintf("%04d", uuid.New().ID()%10000)
}

// GenerateLoginOTP generates a 6-digit OTP for login
func GenerateLoginOTP() string {
	return fmt.Sprintf("%06d", uuid.New().ID()%1000000)
}
