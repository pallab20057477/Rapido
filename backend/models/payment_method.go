package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CardType represents the card type
type CardType string

const (
	CardTypeCredit CardType = "credit"
	CardTypeDebit  CardType = "debit"
)

// CardBrand represents the card brand
type CardBrand string

const (
	CardBrandVisa       CardBrand = "visa"
	CardBrandMastercard CardBrand = "mastercard"
	CardBrandAmex       CardBrand = "amex"
	CardBrandRuPay      CardBrand = "rupay"
	CardBrandDiscover   CardBrand = "discover"
)

// PaymentMethod represents a saved payment method for a user
type PaymentMethod struct {
	ID        uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;"`
	UserID    uuid.UUID       `json:"user_id" gorm:"type:uuid;not null;index"`
	Type      string          `json:"type" gorm:"not null"` // card, upi, wallet, cash
	IsDefault bool            `json:"is_default" gorm:"default:false"`
	Status    string          `json:"status" gorm:"default:active"` // active, expired, disabled
	Nickname  string          `json:"nickname"`
	CreatedAt time.Time       `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time       `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt *gorm.DeletedAt `json:"-" gorm:"index"`

	// Card specific fields (encrypted)
	CardDetails *CardDetails `json:"card_details,omitempty" gorm:"-"`

	// UPI specific fields (encrypted)
	UPIDetails *UPIDetails `json:"upi_details,omitempty" gorm:"-"`
}

// CardDetails represents saved card information (stored encrypted)
type CardDetails struct {
	CardNumber     string    `json:"card_number" gorm:"-"`  // Encrypted at rest
	Last4          string    `json:"last4" gorm:"not null"` // Last 4 digits (unencrypted for display)
	CardType       CardType  `json:"card_type" gorm:"not null"`
	CardBrand      CardBrand `json:"card_brand" gorm:"not null"`
	ExpiryMonth    int       `json:"expiry_month" gorm:"not null"`
	ExpiryYear     int       `json:"expiry_year" gorm:"not null"`
	CardholderName string    `json:"cardholder_name"`
	Token          string    `json:"-" gorm:"column:card_token"` // Payment gateway token
	BillingAddress string    `json:"billing_address"`
	IsAutoDebit    bool      `json:"is_auto_debit" gorm:"default:false"` // For subscription/repeat payments
}

// TableName specifies the table name for PaymentMethod
func (PaymentMethod) TableName() string {
	return "payment_methods"
}

// UPIDetails represents saved UPI information
type UPIDetails struct {
	VPA         string `json:"vpa" gorm:"not null"` // Virtual Payment Address (UPI ID)
	BankName    string `json:"bank_name"`
	BankAccount string `json:"bank_account" gorm:"-"` // Encrypted
	AccountType string `json:"account_type"`          // savings, current
	IFSCCode    string `json:"ifsc_code" gorm:"-"`    // Encrypted
	IsAutoDebit bool   `json:"is_auto_debit" gorm:"default:false"`
}

// TableName specifies the table name for UPIDetails
func (UPIDetails) TableName() string {
	return "upi_details"
}

// IsExpired checks if the card is expired
func (pm *PaymentMethod) IsExpired() bool {
	if pm.Type != "card" || pm.CardDetails == nil {
		return false
	}

	now := time.Now()
	cardExpiry := time.Date(pm.CardDetails.ExpiryYear, time.Month(pm.CardDetails.ExpiryMonth), 1, 0, 0, 0, 0, time.UTC)
	// Card expires at end of expiry month
	cardExpiry = cardExpiry.AddDate(0, 1, -1)

	return now.After(cardExpiry)
}

// MaskCardNumber returns masked card number for display
func (cd *CardDetails) MaskCardNumber() string {
	if cd.Last4 == "" {
		return "****"
	}
	return "**** **** **** " + cd.Last4
}

// ValidateUPI validates UPI VPA format
func (upi *UPIDetails) ValidateUPI() bool {
	// Basic UPI validation: should contain @ and be alphanumeric before @
	if upi.VPA == "" {
		return false
	}
	// VPA format: user@upi (e.g., user@okaxis, user@paytm)
	return len(upi.VPA) > 3 && contains(upi.VPA, "@")
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && findSubstr(s, substr) >= 0
}

func findSubstr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// AddPaymentMethodRequest represents request to add payment method
type AddPaymentMethodRequest struct {
	Type         string `json:"type" binding:"required,oneof=card upi"`
	Nickname     string `json:"nickname"`
	SetAsDefault bool   `json:"set_as_default"`

	// For cards
	CardNumber     string   `json:"card_number,omitempty"`
	ExpiryMonth    int      `json:"expiry_month,omitempty"`
	ExpiryYear     int      `json:"expiry_year,omitempty"`
	CVV            string   `json:"cvv,omitempty"`
	CardholderName string   `json:"cardholder_name,omitempty"`
	CardType       CardType `json:"card_type,omitempty"`
	BillingAddress string   `json:"billing_address,omitempty"`

	// For UPI
	VPA string `json:"vpa,omitempty"`
}

// PaymentMethodResponse represents payment method response
type PaymentMethodResponse struct {
	ID        uuid.UUID `json:"id"`
	Type      string    `json:"type"`
	IsDefault bool      `json:"is_default"`
	Status    string    `json:"status"`
	Nickname  string    `json:"nickname"`
	CreatedAt time.Time `json:"created_at"`

	// Card details (masked)
	CardLast4   string    `json:"card_last4,omitempty"`
	CardBrand   CardBrand `json:"card_brand,omitempty"`
	CardType    CardType  `json:"card_type,omitempty"`
	ExpiryMonth int       `json:"expiry_month,omitempty"`
	ExpiryYear  int       `json:"expiry_year,omitempty"`

	// UPI details
	VPAMasked string `json:"vpa_masked,omitempty"`
	BankName  string `json:"bank_name,omitempty"`
}

