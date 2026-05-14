package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Payment status constants
const (
	PaymentStatusPending   = "pending"
	PaymentStatusCompleted = "completed"
	PaymentStatusFailed    = "failed"
	PaymentStatusRefunded  = "refunded"
	PaymentStatusDisputed  = "disputed"
)

// Payment method constants
const (
	PaymentMethodCash   = "cash"
	PaymentMethodUPI    = "upi"
	PaymentMethodCard   = "card"
	PaymentMethodWallet = "wallet"
)

// Transaction types
const (
	TransactionTypeRidePayment      = "ride_payment"
	TransactionTypeWalletTopup      = "wallet_topup"
	TransactionTypeWalletWithdrawal = "wallet_withdrawal"
	TransactionTypeDriverPayout     = "driver_payout"
	TransactionTypeRefund           = "refund"
	TransactionTypePenalty          = "penalty"
	TransactionTypeCommission       = "commission"
)

type Payment struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	RideID         uuid.UUID      `json:"ride_id" gorm:"not null;index"`
	Ride           *Ride          `json:"ride,omitempty" gorm:"foreignKey:RideID"`
	PayerID        uuid.UUID      `json:"payer_id" gorm:"not null;index"`
	PayeeID        uuid.UUID      `json:"payee_id" gorm:"not null;index"`
	Amount         float64        `json:"amount" gorm:"not null"`
	Currency       string         `json:"currency" gorm:"default:INR"`
	Method         string         `json:"method" gorm:"not null"`
	Status         string         `json:"status" gorm:"default:pending"`
	TransactionID  string         `json:"transaction_id,omitempty" gorm:"uniqueIndex"`
	Gateway        string         `json:"gateway,omitempty"` // razorpay, stripe, etc.
	GatewayRef     string         `json:"gateway_ref,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty" gorm:"uniqueIndex"`
	FailureReason  string         `json:"failure_reason,omitempty"`
	RefundedAt     *time.Time     `json:"refunded_at,omitempty"`
	RefundAmount   float64        `json:"refund_amount" gorm:"default:0"`
	Metadata       JSONMap        `json:"metadata,omitempty" gorm:"type:jsonb"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

type Transaction struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	UserID        uuid.UUID  `json:"user_id" gorm:"not null;index"`
	Type          string     `json:"type" gorm:"not null"`
	Amount        float64    `json:"amount" gorm:"not null"`
	Currency      string     `json:"currency" gorm:"default:INR"`
	Status        string     `json:"status" gorm:"default:pending"`
	Description   string     `json:"description,omitempty"`
	ReferenceID   string     `json:"reference_id,omitempty"` // ride_id, withdrawal_id, etc.
	PaymentID     *uuid.UUID `json:"payment_id,omitempty"`
	WalletBalance float64    `json:"wallet_balance"`
	Metadata      JSONMap    `json:"metadata,omitempty" gorm:"type:jsonb"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Wallet struct {
	ID                 uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	UserID             uuid.UUID      `json:"user_id" gorm:"uniqueIndex;not null"`
	Balance            float64        `json:"balance" gorm:"default:0"`
	Currency           string         `json:"currency" gorm:"default:INR"`
	IsActive           bool           `json:"is_active" gorm:"default:true"`
	DailyLimit         float64        `json:"daily_limit" gorm:"default:10000"`
	MonthlyLimit       float64        `json:"monthly_limit" gorm:"default:100000"`
	AutoRecharge       bool           `json:"auto_recharge" gorm:"default:false"`
	AutoRechargeAmount float64        `json:"auto_recharge_amount" gorm:"default:0"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `json:"-" gorm:"index"`
}

type Commission struct {
	ID                 uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	RideID             uuid.UUID  `json:"ride_id" gorm:"not null;uniqueIndex"`
	DriverID           uuid.UUID  `json:"driver_id" gorm:"not null;index"`
	TotalFare          float64    `json:"total_fare"`
	PlatformCommission float64    `json:"platform_commission"`
	DriverEarnings     float64    `json:"driver_earnings"`
	TaxAmount          float64    `json:"tax_amount"`
	ServiceFee         float64    `json:"service_fee"`
	PlatformPercent    float64    `json:"platform_percent"`
	PaidAt             *time.Time `json:"paid_at,omitempty"`
	SettlementID       *string    `json:"settlement_id,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type Withdrawal struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	DriverID        uuid.UUID  `json:"driver_id" gorm:"not null;index"`
	Amount          float64    `json:"amount" gorm:"not null"`
	Currency        string     `json:"currency" gorm:"default:INR"`
	Status          string     `json:"status" gorm:"default:pending"` // pending, processing, completed, rejected
	Method          string     `json:"method" gorm:"not null"`          // bank_transfer, upi
	BankDetails     JSONMap    `json:"bank_details,omitempty" gorm:"type:jsonb"`
	ProcessedAt     *time.Time `json:"processed_at,omitempty"`
	ProcessedBy     *uuid.UUID `json:"processed_by,omitempty"`
	RejectionReason string     `json:"rejection_reason,omitempty"`
	TransactionID   *uuid.UUID `json:"transaction_id,omitempty"`
	RequestedAt     time.Time  `json:"requested_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Invoice represents a generated invoice for completed rides
type Invoice struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;"`
	RideID        uuid.UUID  `json:"ride_id" gorm:"not null;uniqueIndex"`
	PaymentID     uuid.UUID  `json:"payment_id" gorm:"not null"`
	InvoiceNumber string     `json:"invoice_number" gorm:"uniqueIndex;not null"`
	CustomerName  string     `json:"customer_name"`
	CustomerGST   string     `json:"customer_gst,omitempty"`
	Amount        float64    `json:"amount"`
	TaxAmount     float64    `json:"tax_amount"`
	TotalAmount   float64    `json:"total_amount"`
	GSTPercent    float64    `json:"gst_percent" gorm:"default:18"`
	InvoiceURL    string     `json:"invoice_url,omitempty"`
	GeneratedAt   time.Time  `json:"generated_at"`
	SentAt        *time.Time `json:"sent_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// JSONMap for storing flexible JSON data
type JSONMap map[string]interface{}

func (p *Payment) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (t *Transaction) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

func (w *Wallet) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

func (c *Commission) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

func (w *Withdrawal) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

func (i *Invoice) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

