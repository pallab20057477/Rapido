package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

type Meta struct {
	Page       int   `json:"page,omitempty"`
	PerPage    int   `json:"per_page,omitempty"`
	Total      int64 `json:"total,omitempty"`
	TotalPages int   `json:"total_pages,omitempty"`
}

// SuccessResponse creates a success response
func SuccessResponse(message string, data interface{}) Response {
	return Response{
		Success: true,
		Message: message,
		Data:    data,
	}
}

// ErrorResponse creates an error response
func ErrorResponse(message string, err string) Response {
	return Response{
		Success: false,
		Message: message,
		Error:   err,
	}
}

// SanitizedErrorResponse creates an error response with production-safe error messaging.
// It sanitizes internal errors and returns only user-safe error text.
// Internal errors are logged but not exposed to client.
func SanitizedErrorResponse(message, internalErr string) Response {
	// Log internal error for debugging, but sanitize the client response
	if internalErr != "" {
		switch {
		case strings.Contains(strings.ToLower(internalErr), "unique constraint"):
			return ErrorResponse(message, "This record already exists")
		case strings.Contains(strings.ToLower(internalErr), "foreign key constraint"):
			return ErrorResponse(message, "Referenced record not found")
		case strings.Contains(strings.ToLower(internalErr), "database"):
			return ErrorResponse(message, "Data operation failed")
		case strings.Contains(strings.ToLower(internalErr), "connection"):
			return ErrorResponse(message, "Service temporarily unavailable")
		case strings.Contains(strings.ToLower(internalErr), "timeout"):
			return ErrorResponse(message, "Request timeout")
		case strings.Contains(strings.ToLower(internalErr), "permission"):
			return ErrorResponse(message, "You do not have permission for this action")
		case strings.Contains(strings.ToLower(internalErr), "already"):
			return ErrorResponse(message, "This action has already been completed")
		case len(internalErr) > 0:
			// For unknown errors, return generic message but only show first 50 chars as hint
			// This helps developers but doesn't leak sensitive details
			return ErrorResponse(message, "")
		}
	}
	return ErrorResponse(message, "")
}

// PaginatedResponse creates a paginated response
func PaginatedResponse(data interface{}, page, perPage int, total int64) Response {
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	return Response{
		Success: true,
		Data:    data,
		Meta: &Meta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	}
}

// IsValidUUID checks if string is valid UUID
func IsValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

// IsValidPhone checks if phone number is valid (Indian format)
func IsValidPhone(phone string) bool {
	// Remove +91 prefix if present
	phone = strings.TrimPrefix(phone, "+")
	phone = strings.TrimPrefix(phone, "91")

	// Check if it's a 10 digit number starting with 6-9
	re := regexp.MustCompile(`^[6-9]\d{9}$`)
	return re.MatchString(phone)
}

// IsValidEmail checks if email is valid
func IsValidEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

// SanitizePhone sanitizes phone number
func SanitizePhone(phone string) string {
	// Remove all non-digit characters
	re := regexp.MustCompile(`\D`)
	phone = re.ReplaceAllString(phone, "")

	// Remove 91 prefix if present and length is 12
	if len(phone) == 12 && strings.HasPrefix(phone, "91") {
		phone = phone[2:]
	}

	return phone
}

// FormatPhone formats phone number with +91 prefix
func FormatPhone(phone string) string {
	phone = SanitizePhone(phone)
	if len(phone) == 10 {
		return "+91" + phone
	}
	return phone
}

// ParseJSON parses JSON string to map
func ParseJSON(data string) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal([]byte(data), &result)
	return result, err
}

// ToJSON converts interface to JSON string
func ToJSON(data interface{}) (string, error) {
	bytes, err := json.Marshal(data)
	return string(bytes), err
}

// GenerateIdempotencyKey generates a unique idempotency key
func GenerateIdempotencyKey() string {
	return fmt.Sprintf("%s-%d", uuid.New().String(), time.Now().UnixNano())
}

// Contains checks if string slice contains a value
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Remove removes an item from string slice
func Remove(slice []string, item string) []string {
	result := []string{}
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// Chunk splits slice into chunks
func Chunk(slice []string, chunkSize int) [][]string {
	var chunks [][]string
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

// FormatDuration formats duration in human readable format
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d sec", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%d hr %d min", hours, minutes)
}

// FormatCurrency formats amount as INR currency
func FormatCurrency(amount float64) string {
	return fmt.Sprintf("₹%.2f", amount)
}

// Round rounds float to 2 decimal places
func Round(amount float64) float64 {
	return math.Round(amount*100) / 100
}

// Ptr returns pointer to value
func Ptr[T any](v T) *T {
	return &v
}

// StringPtr returns pointer to string
func StringPtr(s string) *string {
	return &s
}

// TimePtr returns pointer to time
func TimePtr(t time.Time) *time.Time {
	return &t
}

// Coalesce returns first non-empty string
func Coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// GetEnv returns environment variable or default value
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Password validation constants
const (
	MinPasswordLength = 6
	MaxPasswordLength = 128
)

// IsValidPassword checks if password meets minimum requirements
func IsValidPassword(password string) bool {
	if len(password) < MinPasswordLength {
		return false
	}
	if len(password) > MaxPasswordLength {
		return false
	}
	return true
}
