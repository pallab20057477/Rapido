package services

import (
	"errors"
	"fmt"

	"rapido-backend/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LedgerService struct{}

func NewLedgerService() *LedgerService {
	return &LedgerService{}
}

func ledgerAccountKey(accountType string, ownerID *uuid.UUID, currency string) string {
	if ownerID == nil || *ownerID == uuid.Nil {
		return fmt.Sprintf("%s:%s", accountType, currency)
	}
	return fmt.Sprintf("%s:%s:%s", accountType, ownerID.String(), currency)
}

func (s *LedgerService) ensureAccount(tx *gorm.DB, accountType string, ownerID *uuid.UUID, currency string) (*models.LedgerAccount, error) {
	key := ledgerAccountKey(accountType, ownerID, currency)
	var account models.LedgerAccount
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("account_key = ?", key).First(&account).Error; err == nil {
		return &account, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	account = models.LedgerAccount{
		AccountKey:  key,
		AccountType: accountType,
		OwnerID:     ownerID,
		Currency:    currency,
		Balance:     0,
		IsActive:    true,
	}
	if err := tx.Create(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// checkIdempotency checks if entry already exists (double-spend protection)
func (s *LedgerService) checkIdempotency(tx *gorm.DB, referenceType, referenceID string, direction string) (bool, error) {
	var count int64
	result := tx.Model(&models.LedgerEntry{}).
		Where("reference_type = ? AND reference_id = ? AND direction = ?",
			referenceType, referenceID, direction).
		Count(&count)
	return count > 0, result.Error
}

func (s *LedgerService) postEntry(tx *gorm.DB, accountType string, ownerID *uuid.UUID, currency string, direction string, amount float64, referenceType string, referenceID string, description string, batchID uuid.UUID) (*models.LedgerEntry, error) {
	// Double-spend protection: Check if entry already posted
	exists, err := s.checkIdempotency(tx, referenceType, referenceID, direction)
	if err != nil {
		return nil, fmt.Errorf("idempotency check failed: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("duplicate ledger entry: %s:%s already posted", referenceType, referenceID)
	}
	if amount < 0 {
		return nil, errors.New("ledger amount must be positive")
	}

	account, err := s.ensureAccount(tx, accountType, ownerID, currency)
	if err != nil {
		return nil, err
	}

	balanceBefore := account.Balance
	balanceAfter := balanceBefore
	switch direction {
	case models.LedgerDirectionCredit:
		balanceAfter += amount
	case models.LedgerDirectionDebit:
		balanceAfter -= amount
	default:
		return nil, errors.New("invalid ledger direction")
	}

	account.Balance = balanceAfter
	if err := tx.Save(account).Error; err != nil {
		return nil, err
	}

	entry := &models.LedgerEntry{
		BatchID:       batchID,
		AccountID:     account.ID,
		Direction:     direction,
		Amount:        amount,
		BalanceBefore: balanceBefore,
		BalanceAfter:  balanceAfter,
		Currency:      currency,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
		Description:   description,
	}
	if err := tx.Create(entry).Error; err != nil {
		return nil, err
	}

	return entry, nil
}

func (s *LedgerService) RecordTopup(tx *gorm.DB, userID uuid.UUID, amount float64, referenceID string) error {
	batchID := uuid.New()
	currency := "INR"
	if _, err := s.postEntry(tx, models.LedgerAccountTypeTopupClearing, nil, currency, models.LedgerDirectionDebit, amount, "wallet_topup", referenceID, "Wallet topup clearing", batchID); err != nil {
		return err
	}
	_, err := s.postEntry(tx, models.LedgerAccountTypeUserWallet, &userID, currency, models.LedgerDirectionCredit, amount, "wallet_topup", referenceID, "Wallet topup", batchID)
	return err
}

func (s *LedgerService) RecordRideSettlement(tx *gorm.DB, sourceAccountType string, sourceAccountID *uuid.UUID, riderID, driverID uuid.UUID, riderDebit, driverCredit, platformCredit float64, referenceID string) error {
	batchID := uuid.New()
	currency := "INR"

	if riderDebit > 0 {
		if _, err := s.postEntry(tx, sourceAccountType, sourceAccountID, currency, models.LedgerDirectionDebit, riderDebit, "ride_payment", referenceID, "Ride payment debit", batchID); err != nil {
			return err
		}
	}

	if driverCredit > 0 {
		if _, err := s.postEntry(tx, models.LedgerAccountTypeDriverWallet, &driverID, currency, models.LedgerDirectionCredit, driverCredit, "ride_payment", referenceID, "Driver payout credit", batchID); err != nil {
			return err
		}
	}

	if platformCredit > 0 {
		if _, err := s.postEntry(tx, models.LedgerAccountTypePlatformRevenue, nil, currency, models.LedgerDirectionCredit, platformCredit, "ride_payment", referenceID, "Platform commission", batchID); err != nil {
			return err
		}
	}

	return nil
}

func (s *LedgerService) RecordRefund(tx *gorm.DB, userID uuid.UUID, amount float64, referenceID string) error {
	batchID := uuid.New()
	currency := "INR"
	if _, err := s.postEntry(tx, models.LedgerAccountTypeRefundClearing, nil, currency, models.LedgerDirectionDebit, amount, "refund", referenceID, "Refund clearing", batchID); err != nil {
		return err
	}
	_, err := s.postEntry(tx, models.LedgerAccountTypeUserWallet, &userID, currency, models.LedgerDirectionCredit, amount, "refund", referenceID, "Refund credit", batchID)
	return err
}

func (s *LedgerService) RecordWithdrawal(tx *gorm.DB, driverID uuid.UUID, amount float64, referenceID string) error {
	batchID := uuid.New()
	currency := "INR"
	if _, err := s.postEntry(tx, models.LedgerAccountTypeDriverEarnings, &driverID, currency, models.LedgerDirectionDebit, amount, "withdrawal", referenceID, "Driver earnings withdrawal debit", batchID); err != nil {
		return err
	}
	_, err := s.postEntry(tx, models.LedgerAccountTypeWithdrawalClearing, nil, currency, models.LedgerDirectionCredit, amount, "withdrawal", referenceID, "Withdrawal clearing credit", batchID)
	return err
}
