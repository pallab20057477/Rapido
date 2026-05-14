package services

import (
	"fmt"
	"log"
	"strings"

	"rapido-backend/database"
	"rapido-backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PaymentMethodService handles saved payment method operations
type PaymentMethodService struct {
	db *gorm.DB
}

// NewPaymentMethodService creates a new service
func NewPaymentMethodService() *PaymentMethodService {
	return &PaymentMethodService{
		db: database.DB,
	}
}

// AddCard adds a new saved card
func (s *PaymentMethodService) AddCard(userID uuid.UUID, req models.AddPaymentMethodRequest) (*models.PaymentMethod, error) {
	// Validate card details
	if req.CardNumber == "" || req.CVV == "" {
		return nil, fmt.Errorf("card number and CVV are required")
	}

	if req.ExpiryMonth < 1 || req.ExpiryMonth > 12 {
		return nil, fmt.Errorf("invalid expiry month")
	}

	// Check card limit (max 5 cards per user)
	var count int64
	if err := s.db.Model(&models.PaymentMethod{}).Where("user_id = ? AND type = ?", userID, models.PaymentMethodCard).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("failed to count cards: %w", err)
	}
	if count >= 5 {
		return nil, fmt.Errorf("maximum 5 cards allowed per user")
	}

	// Extract last 4 digits
	last4 := ""
	if len(req.CardNumber) >= 4 {
		last4 = req.CardNumber[len(req.CardNumber)-4:]
	}

	// Detect card brand
	brand := s.detectCardBrand(req.CardNumber)

	// Create payment method
	pm := &models.PaymentMethod{
		UserID:    userID,
		Type:      models.PaymentMethodCard,
		Nickname:  req.Nickname,
		Status:    "active",
		IsDefault: req.SetAsDefault,
		CardDetails: &models.CardDetails{
			Last4:          last4,
			CardType:       req.CardType,
			CardBrand:      brand,
			ExpiryMonth:    req.ExpiryMonth,
			ExpiryYear:     req.ExpiryYear,
			CardholderName: req.CardholderName,
			BillingAddress: req.BillingAddress,
		},
	}

	if req.SetAsDefault {
		// Unset current default
		s.db.Model(&models.PaymentMethod{}).Where("user_id = ? AND is_default = ?", userID, true).Update("is_default", false)
	}

	if err := s.db.Create(pm).Error; err != nil {
		return nil, fmt.Errorf("failed to save card: %w", err)
	}

	log.Printf("[PaymentMethod] Added card %s for user %s", pm.ID, userID)
	return pm, nil
}

// AddUPI adds a new saved UPI ID
func (s *PaymentMethodService) AddUPI(userID uuid.UUID, req models.AddPaymentMethodRequest) (*models.PaymentMethod, error) {
	// Validate UPI
	if req.VPA == "" {
		return nil, fmt.Errorf("UPI VPA is required")
	}

	// Basic UPI format validation
	if !strings.Contains(req.VPA, "@") {
		return nil, fmt.Errorf("invalid UPA format")
	}

	// Check UPI limit (max 3 UPI IDs per user)
	var count int64
	if err := s.db.Model(&models.PaymentMethod{}).Where("user_id = ? AND type = ?", userID, models.PaymentMethodUPI).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("failed to count UPI IDs: %w", err)
	}
	if count >= 3 {
		return nil, fmt.Errorf("maximum 3 UPI IDs allowed per user")
	}

	// Check for duplicate UPI
	var existing models.PaymentMethod
	if err := s.db.Where("user_id = ? AND type = ?", userID, models.PaymentMethodUPI).First(&existing).Error; err == nil {
		// Check if UPI already exists
		// Would need to decrypt to check, for now we allow duplicates
	}

	// Detect bank from UPA (simple heuristic)
	bankName := s.detectBankFromVPA(req.VPA)

	pm := &models.PaymentMethod{
		UserID:    userID,
		Type:      models.PaymentMethodUPI,
		Nickname:  req.Nickname,
		Status:    "active",
		IsDefault: req.SetAsDefault,
		UPIDetails: &models.UPIDetails{
			VPA:      req.VPA,
			BankName: bankName,
		},
	}

	if req.SetAsDefault {
		s.db.Model(&models.PaymentMethod{}).Where("user_id = ? AND is_default = ?", userID, true).Update("is_default", false)
	}

	if err := s.db.Create(pm).Error; err != nil {
		return nil, fmt.Errorf("failed to save UPI: %w", err)
	}

	log.Printf("[PaymentMethod] Added UPI %s for user %s", pm.ID, userID)
	return pm, nil
}

// GetPaymentMethods gets all payment methods for a user
func (s *PaymentMethodService) GetPaymentMethods(userID uuid.UUID) ([]models.PaymentMethod, error) {
	var methods []models.PaymentMethod
	if err := s.db.Where("user_id = ? AND status = ?", userID, "active").
		Order("is_default DESC, created_at DESC").
		Find(&methods).Error; err != nil {
		return nil, fmt.Errorf("failed to get payment methods: %w", err)
	}
	return methods, nil
}

// GetPaymentMethodByID gets a specific payment method
func (s *PaymentMethodService) GetPaymentMethodByID(methodID, userID uuid.UUID) (*models.PaymentMethod, error) {
	var method models.PaymentMethod
	if err := s.db.Where("id = ? AND user_id = ?", methodID, userID).First(&method).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("payment method not found")
		}
		return nil, fmt.Errorf("failed to get payment method: %w", err)
	}
	return &method, nil
}

// RemovePaymentMethod removes a payment method
func (s *PaymentMethodService) RemovePaymentMethod(methodID, userID uuid.UUID) error {
	result := s.db.Where("id = ? AND user_id = ?", methodID, userID).Delete(&models.PaymentMethod{})
	if result.Error != nil {
		return fmt.Errorf("failed to remove payment method: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("payment method not found")
	}
	return nil
}

// SetDefaultPaymentMethod sets a payment method as default
func (s *PaymentMethodService) SetDefaultPaymentMethod(methodID, userID uuid.UUID) error {
	// First, unset current default
	if err := s.db.Model(&models.PaymentMethod{}).Where("user_id = ? AND is_default = ?", userID, true).Update("is_default", false).Error; err != nil {
		return fmt.Errorf("failed to unset default: %w", err)
	}

	// Set new default
	result := s.db.Model(&models.PaymentMethod{}).Where("id = ? AND user_id = ?", methodID, userID).Update("is_default", true)
	if result.Error != nil {
		return fmt.Errorf("failed to set default: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("payment method not found")
	}

	return nil
}

// detectCardBrand detects card brand from card number
func (s *PaymentMethodService) detectCardBrand(cardNumber string) models.CardBrand {
	if len(cardNumber) == 0 {
		return ""
	}

	// Simple prefix-based detection
	switch {
	case strings.HasPrefix(cardNumber, "4"):
		return models.CardBrandVisa
	case strings.HasPrefix(cardNumber, "5"):
		return models.CardBrandMastercard
	case strings.HasPrefix(cardNumber, "37") || strings.HasPrefix(cardNumber, "34"):
		return models.CardBrandAmex
	case strings.HasPrefix(cardNumber, "60") || strings.HasPrefix(cardNumber, "65"):
		return models.CardBrandRuPay
	default:
		return ""
	}
}

// detectBankFromVPA detects bank from UPI VPA
func (s *PaymentMethodService) detectBankFromVPA(vpa string) string {
	// Extract handle (part after @)
	parts := strings.Split(vpa, "@")
	if len(parts) != 2 {
		return ""
	}
	handle := parts[1]

	// Map handles to bank names
	bankMap := map[string]string{
		"okaxis":    "Axis Bank",
		"okhdfcbank": "HDFC Bank",
		"oksbi":     "SBI",
		"okicici":   "ICICI Bank",
		"paytm":     "Paytm",
		"upi":       "Bank",
	}

	if bankName, ok := bankMap[handle]; ok {
		return bankName
	}
	return "Bank"
}

// ToResponse converts PaymentMethod to PaymentMethodResponse
func (s *PaymentMethodService) ToResponse(pm models.PaymentMethod) models.PaymentMethodResponse {
	resp := models.PaymentMethodResponse{
		ID:        pm.ID,
		Type:      pm.Type,
		IsDefault: pm.IsDefault,
		Status:    pm.Status,
		Nickname:  pm.Nickname,
		CreatedAt: pm.CreatedAt,
	}

	if pm.Type == models.PaymentMethodCard && pm.CardDetails != nil {
		resp.CardLast4 = pm.CardDetails.Last4
		resp.CardBrand = pm.CardDetails.CardBrand
		resp.CardType = pm.CardDetails.CardType
		resp.ExpiryMonth = pm.CardDetails.ExpiryMonth
		resp.ExpiryYear = pm.CardDetails.ExpiryYear
	}

	if pm.Type == models.PaymentMethodUPI && pm.UPIDetails != nil {
		// Mask UPI: show first 3 chars and domain
		vpa := pm.UPIDetails.VPA
		if len(vpa) > 3 {
			parts := strings.Split(vpa, "@")
			if len(parts) == 2 {
				resp.VPAMasked = vpa[:3] + "***@" + parts[1]
			}
		}
		resp.BankName = pm.UPIDetails.BankName
	}

	return resp
}
