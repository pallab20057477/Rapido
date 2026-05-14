package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"rapido-backend/config"
)

// RazorpayGateway implements the payment gateway interface for Razorpay
type RazorpayGateway struct {
	keyID     string
	keySecret string
	client    *http.Client
	baseURL   string
}

// RazorpayOrder represents a Razorpay order
type RazorpayOrder struct {
	ID         string            `json:"id"`
	Entity     string            `json:"entity"`
	Amount     int               `json:"amount"`
	AmountPaid int               `json:"amount_paid"`
	AmountDue  int               `json:"amount_due"`
	Currency   string            `json:"currency"`
	Receipt    string            `json:"receipt"`
	Status     string            `json:"status"`
	Attempts   int               `json:"attempts"`
	CreatedAt  int64             `json:"created_at"`
	Notes      map[string]string `json:"notes"`
}

// RazorpayRefund represents a refund from Razorpay
type RazorpayRefund struct {
	ID        string `json:"id"`
	Entity    string `json:"entity"`
	PaymentID string `json:"payment_id"`
	Amount    int    `json:"amount"`
	Currency  string `json:"currency"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

// NewRazorpayGateway creates a new Razorpay gateway instance
func NewRazorpayGateway() *RazorpayGateway {
	cfg := config.Get()
	return &RazorpayGateway{
		keyID:     cfg.Razorpay.KeyID,
		keySecret: cfg.Razorpay.KeySecret,
		client:    &http.Client{Timeout: 30 * time.Second},
		baseURL:   "https://api.razorpay.com/v1",
	}
}

// IsConfigured returns true if Razorpay credentials are configured
func (r *RazorpayGateway) IsConfigured() bool {
	return r.keyID != "" && r.keySecret != ""
}

// CreateOrder creates a new order in Razorpay
func (r *RazorpayGateway) CreateOrder(ctx context.Context, amount float64, currency, receipt string, notes map[string]string) (*RazorpayOrder, error) {
	if !r.IsConfigured() {
		return nil, fmt.Errorf("razorpay not configured")
	}

	// Convert amount to paise (Razorpay expects smallest currency unit)
	amountInPaise := int(amount * 100)

	payload := map[string]interface{}{
		"amount":          amountInPaise,
		"currency":        currency,
		"receipt":         receipt,
		"notes":           notes,
		"partial_payment": false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal order payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.baseURL+"/orders", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(r.keyID, r.keySecret)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("razorpay error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var order RazorpayOrder
	if err := json.Unmarshal(respBody, &order); err != nil {
		return nil, fmt.Errorf("failed to parse order response: %w", err)
	}

	return &order, nil
}

// CapturePayment captures a payment in Razorpay
func (r *RazorpayGateway) CapturePayment(ctx context.Context, paymentID string, amount float64, currency string) (*RazorpayPayment, error) {
	if !r.IsConfigured() {
		return nil, fmt.Errorf("razorpay not configured")
	}

	amountInPaise := int(amount * 100)

	payload := map[string]interface{}{
		"amount":   amountInPaise,
		"currency": currency,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal capture payload: %w", err)
	}

	url := fmt.Sprintf("%s/payments/%s/capture", r.baseURL, paymentID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(r.keyID, r.keySecret)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to capture payment: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var payment RazorpayPayment
	if err := json.Unmarshal(respBody, &payment); err != nil {
		return nil, fmt.Errorf("failed to parse payment response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if payment.ErrorCode != "" {
			return nil, fmt.Errorf("razorpay error: %s - %s", payment.ErrorCode, payment.ErrorDescription)
		}
		return nil, fmt.Errorf("razorpay error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return &payment, nil
}

// GetPayment fetches payment details from Razorpay
// Returns payment data as the shared RazorpayPayment type from payment_reconciliation.go
func (r *RazorpayGateway) GetPayment(ctx context.Context, paymentID string) (*RazorpayPayment, error) {
	if !r.IsConfigured() {
		return nil, fmt.Errorf("razorpay not configured")
	}

	url := fmt.Sprintf("%s/payments/%s", r.baseURL, paymentID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(r.keyID, r.keySecret)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch payment: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("razorpay error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var payment RazorpayPayment
	if err := json.Unmarshal(respBody, &payment); err != nil {
		return nil, fmt.Errorf("failed to parse payment response: %w", err)
	}

	return &payment, nil
}

// CreateRefund creates a refund for a payment
func (r *RazorpayGateway) CreateRefund(ctx context.Context, paymentID string, amount float64, notes map[string]string) (*RazorpayRefund, error) {
	if !r.IsConfigured() {
		return nil, fmt.Errorf("razorpay not configured")
	}

	amountInPaise := int(amount * 100)
	payload := map[string]interface{}{
		"amount": amountInPaise,
		"notes":  notes,
		"speed":  "normal", // or "optimum" for instant refund
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal refund payload: %w", err)
	}

	url := fmt.Sprintf("%s/payments/%s/refund", r.baseURL, paymentID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(r.keyID, r.keySecret)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create refund: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("razorpay error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var refund RazorpayRefund
	if err := json.Unmarshal(respBody, &refund); err != nil {
		return nil, fmt.Errorf("failed to parse refund response: %w", err)
	}

	return &refund, nil
}

// VerifyWebhookSignature verifies the Razorpay webhook signature
func (r *RazorpayGateway) VerifyWebhookSignature(body []byte, signature, secret string) bool {
	// Razorpay webhook signature format: HMAC-SHA256 of body with webhook secret
	expectedSig := r.calculateSignature(body, secret)
	return signature == expectedSig
}

// calculateSignature calculates HMAC-SHA256 signature
func (r *RazorpayGateway) calculateSignature(body []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// FetchAllPayments fetches all payments with optional filters
// Returns slice of the shared RazorpayPayment type from payment_reconciliation.go
func (r *RazorpayGateway) FetchAllPayments(ctx context.Context, from, to int64, count int) ([]RazorpayPayment, error) {
	if !r.IsConfigured() {
		return nil, fmt.Errorf("razorpay not configured")
	}

	url := fmt.Sprintf("%s/payments?count=%d", r.baseURL, count)
	if from > 0 {
		url += fmt.Sprintf("&from=%d", from)
	}
	if to > 0 {
		url += fmt.Sprintf("&to=%d", to)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(r.keyID, r.keySecret)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch payments: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("razorpay error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Items []RazorpayPayment `json:"items"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse payments response: %w", err)
	}

	return result.Items, nil
}

// IsRetryableError determines if a Razorpay error is retryable
func (r *RazorpayGateway) IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Non-retryable errors (business logic errors)
	nonRetryable := []string{
		"BAD_REQUEST_ERROR",
		"BAD_REQUEST",
		"idempotency_key",
		"card_declined",
		"expired_card",
		"incorrect_cvc",
		"processing_error",
		"incorrect_number",
		"insufficient_funds",
		"invalid_expiry_month",
		"invalid_expiry_year",
		"unsupported_currency",
		"international_cards_not_allowed",
	}

	for _, pattern := range nonRetryable {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}

	// Retryable errors (network/transient errors)
	retryable := []string{
		"timeout",
		"connection",
		"temporary",
		"rate limit",
		"gateway_timeout",
		"internal_error",
	}

	for _, pattern := range retryable {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Default: retry on any error (conservative approach)
	return true
}
