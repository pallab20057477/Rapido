package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// GenerateOTP generates a secure random OTP
func GenerateOTP(length int) string {
	if length == 0 {
		length = 6
	}

	max := big.NewInt(0)
	max.Exp(big.NewInt(10), big.NewInt(int64(length)), nil)

	n, err := rand.Int(rand.Reader, max)
	if err == nil {
		return fmt.Sprintf("%0*d", length, n)
	}

	// Fallback: use current time entropy to produce a pseudo-random OTP
	// (rare path; crypto/rand should normally succeed)
	maxVal := int64(1)
	for i := 0; i < length; i++ {
		maxVal *= 10
	}
	val := time.Now().UnixNano() % maxVal
	return fmt.Sprintf("%0*d", length, int(val))
}

// MaskPhone masks phone number for display
func MaskPhone(phone string) string {
	if len(phone) < 4 {
		return phone
	}
	return "****" + phone[len(phone)-4:]
}

// MaskAadhaar masks Aadhaar number
func MaskAadhaar(aadhaar string) string {
	if len(aadhaar) < 4 {
		return aadhaar
	}
	return "XXXX-XXXX-" + aadhaar[len(aadhaar)-4:]
}
