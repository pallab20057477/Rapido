package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	LedgerDirectionDebit  = "debit"
	LedgerDirectionCredit = "credit"
)

const (
	LedgerAccountTypeUserWallet         = "user_wallet"
	LedgerAccountTypeDriverWallet       = "driver_wallet"
	LedgerAccountTypeDriverEarnings     = "driver_earnings"
	LedgerAccountTypePlatformRevenue    = "platform_revenue"
	LedgerAccountTypePaymentClearing    = "payment_clearing"
	LedgerAccountTypeTopupClearing      = "topup_clearing"
	LedgerAccountTypeWithdrawalClearing = "withdrawal_clearing"
	LedgerAccountTypeRefundClearing     = "refund_clearing"
)

type LedgerAccount struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;"`
	AccountKey  string         `json:"account_key" gorm:"uniqueIndex;not null"`
	AccountType string         `json:"account_type" gorm:"not null"`
	OwnerID     *uuid.UUID     `json:"owner_id,omitempty" gorm:"index"`
	Currency    string         `json:"currency" gorm:"default:INR"`
	Balance     float64        `json:"balance" gorm:"default:0"`
	IsActive    bool           `json:"is_active" gorm:"default:true"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

type LedgerEntry struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`
	BatchID       uuid.UUID `json:"batch_id" gorm:"not null;index"`
	AccountID     uuid.UUID `json:"account_id" gorm:"not null;index"`
	Direction     string    `json:"direction" gorm:"not null"`
	Amount        float64   `json:"amount" gorm:"not null"`
	BalanceBefore float64   `json:"balance_before" gorm:"not null"`
	BalanceAfter  float64   `json:"balance_after" gorm:"not null"`
	Currency      string    `json:"currency" gorm:"default:INR"`
	ReferenceType string    `json:"reference_type,omitempty"`
	ReferenceID   string    `json:"reference_id,omitempty" gorm:"index"`
	Description   string    `json:"description,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

func (a *LedgerAccount) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

func (e *LedgerEntry) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.BatchID == uuid.Nil {
		e.BatchID = uuid.New()
	}
	return nil
}

