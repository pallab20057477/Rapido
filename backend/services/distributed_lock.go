package services

import (
	"context"
	"fmt"
	"time"

	"rapido-backend/database"
	"rapido-backend/utils"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// DistributedLock provides Redis-based distributed locking
type DistributedLock struct {
	redis   *redis.Client
	ctx     context.Context
	key     string
	value   string
	ttl     time.Duration
	renewal *time.Ticker
	stopCh  chan bool
}

// LockManager manages distributed locks
type LockManager struct {
	redis *redis.Client
	ctx   context.Context
}

// NewLockManager creates a new lock manager
func NewLockManager() *LockManager {
	return &LockManager{
		redis: database.RedisClient,
		ctx:   context.Background(),
	}
}

// AcquireRideLock attempts to acquire lock for ride acceptance
// Returns (lock, acquired, error)
func (lm *LockManager) AcquireRideLock(rideID string, driverID string, ttl time.Duration) (*DistributedLock, bool, error) {
	lockKey := fmt.Sprintf("lock:ride:%s", rideID)
	lockValue := fmt.Sprintf("%s:%d", driverID, time.Now().UnixNano())
	
	// Use Redis SET NX EX for atomic lock acquisition
	// NX = Only set if not exists
	// EX = Set expiry
	ok, err := lm.redis.SetNX(lm.ctx, lockKey, lockValue, ttl).Result()
	if err != nil {
		return nil, false, fmt.Errorf("failed to acquire lock: %w", err)
	}
	
	if !ok {
		// Lock already held by another driver
		return nil, false, nil
	}
	
	lock := &DistributedLock{
		redis:  lm.redis,
		ctx:    lm.ctx,
		key:    lockKey,
		value:  lockValue,
		ttl:    ttl,
		stopCh: make(chan bool),
	}
	
	// Start background renewal
	lock.startRenewal()
	
	utils.Info("Ride lock acquired",
		zap.String("ride_id", rideID),
		zap.String("driver_id", driverID),
		zap.Duration("ttl", ttl))
	
	return lock, true, nil
}

// Release releases the lock
func (l *DistributedLock) Release() error {
	// Stop renewal goroutine
	if l.renewal != nil {
		l.renewal.Stop()
		close(l.stopCh)
	}
	
	// Use Lua script for atomic check-and-delete
	// Only delete if value matches (prevents deleting someone else's lock)
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	
	result, err := l.redis.Eval(l.ctx, script, []string{l.key}, l.value).Result()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}
	
	if result.(int64) == 1 {
		utils.Info("Ride lock released", zap.String("key", l.key))
	} else {
		utils.Warn("Lock was not released (owned by another process)", zap.String("key", l.key))
	}
	
	return nil
}

// startRenewal starts background lock renewal
func (l *DistributedLock) startRenewal() {
	// Renew at 1/3 of TTL
	renewalInterval := l.ttl / 3
	l.renewal = time.NewTicker(renewalInterval)
	
	go func() {
		for {
			select {
			case <-l.renewal.C:
				// Extend lock TTL
				script := `
					if redis.call("get", KEYS[1]) == ARGV[1] then
						return redis.call("expire", KEYS[1], ARGV[2])
					else
						return 0
					end
				`
				result, err := l.redis.Eval(l.ctx, script, 
					[]string{l.key}, 
					l.value, 
					int(l.ttl.Seconds())).Result()
				
				if err != nil || result.(int64) == 0 {
					utils.Error("Failed to renew lock", zap.Error(err), zap.String("key", l.key))
					return
				}
				
			case <-l.stopCh:
				return
			}
		}
	}()
}

// Extend extends lock TTL manually
func (l *DistributedLock) Extend(additionalTime time.Duration) error {
	newTTL := l.ttl + additionalTime
	
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`
	
	result, err := l.redis.Eval(l.ctx, script, 
		[]string{l.key}, 
		l.value, 
		int(newTTL.Seconds())).Result()
	
	if err != nil {
		return err
	}
	
	if result.(int64) == 0 {
		return fmt.Errorf("lock not owned by this process")
	}
	
	l.ttl = newTTL
	return nil
}

// IsOwner checks if this lock instance still owns the lock
func (l *DistributedLock) IsOwner() bool {
	val, err := l.redis.Get(l.ctx, l.key).Result()
	if err != nil {
		return false
	}
	return val == l.value
}

// GetLockInfo returns current lock holder info
func (lm *LockManager) GetLockInfo(rideID string) (string, error) {
	lockKey := fmt.Sprintf("lock:ride:%s", rideID)
	val, err := lm.redis.Get(lm.ctx, lockKey).Result()
	if err == redis.Nil {
		return "", nil // No lock
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

// ForceUnlock forces unlock (admin only, use with caution)
func (lm *LockManager) ForceUnlock(rideID string) error {
	lockKey := fmt.Sprintf("lock:ride:%s", rideID)
	return lm.redis.Del(lm.ctx, lockKey).Err()
}

// AcquirePaymentLock locks for payment processing
func (lm *LockManager) AcquirePaymentLock(rideID string, ttl time.Duration) (*DistributedLock, bool, error) {
	lockKey := fmt.Sprintf("lock:payment:%s", rideID)
	lockValue := fmt.Sprintf("payment:%d", time.Now().UnixNano())
	
	ok, err := lm.redis.SetNX(lm.ctx, lockKey, lockValue, ttl).Result()
	if err != nil {
		return nil, false, err
	}
	
	if !ok {
		return nil, false, nil
	}
	
	return &DistributedLock{
		redis:  lm.redis,
		ctx:    lm.ctx,
		key:    lockKey,
		value:  lockValue,
		ttl:    ttl,
		stopCh: make(chan bool),
	}, true, nil
}

// AcquireDriverLock locks driver status during ride
func (lm *LockManager) AcquireDriverLock(driverID string, ttl time.Duration) (*DistributedLock, bool, error) {
	lockKey := fmt.Sprintf("lock:driver:%s", driverID)
	lockValue := fmt.Sprintf("busy:%d", time.Now().UnixNano())
	
	ok, err := lm.redis.SetNX(lm.ctx, lockKey, lockValue, ttl).Result()
	if err != nil {
		return nil, false, err
	}
	
	if !ok {
		return nil, false, nil
	}
	
	return &DistributedLock{
		redis:  lm.redis,
		ctx:    lm.ctx,
		key:    lockKey,
		value:  lockValue,
		ttl:    ttl,
		stopCh: make(chan bool),
	}, true, nil
}

// Global instance
var LockMgr *LockManager

// InitLockManager initializes the global lock manager
func InitLockManager() {
	LockMgr = NewLockManager()
	utils.Info("Distributed lock manager initialized")
}
