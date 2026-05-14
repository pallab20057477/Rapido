package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"rapido-backend/config"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client
var Ctx = context.Background()

func ConnectRedis(cfg *config.Config) (*redis.Client, error) {
	// cfg.Redis.Host contains REDIS_ADDR (host:port format)
	addr := cfg.Redis.Host
	// If caller provided only host (e.g. "localhost") and set REDIS_PORT separately,
	// append the port. If nothing provided, default to localhost:6379.
	if addr == "" {
		addr = "localhost"
	}
	if !strings.Contains(addr, ":") {
		// If separate port is configured, use it; otherwise default to 6379
		port := cfg.Redis.Port
		if port == 0 {
			port = 6379
		}
		addr = fmt.Sprintf("%s:%d", addr, port)
	}

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	if err := RedisClient.Ping(Ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return RedisClient, nil
}

// Redis key helpers
func GetDriverLocationKey(driverID string) string {
	return fmt.Sprintf("driver:%s:location", driverID)
}

func GetDriverOnlineKey(driverID string) string {
	return fmt.Sprintf("driver:%s:online", driverID)
}

func GetAvailableDriversKey() string {
	return "drivers:available"
}

func GetRideLockKey(rideID string) string {
	return fmt.Sprintf("ride:%s:lock", rideID)
}

func GetRideDriversNotifiedKey(rideID string) string {
	return fmt.Sprintf("ride:%s:drivers_notified", rideID)
}

func GetDriverCurrentRideKey(driverID string) string {
	return fmt.Sprintf("driver:%s:current_ride", driverID)
}

func GetRiderCurrentRideKey(riderID string) string {
	return fmt.Sprintf("rider:%s:current_ride", riderID)
}

func GetRideStatusKey(rideID string) string {
	return fmt.Sprintf("ride:%s:status", rideID)
}

func GetRideRequestStateKey(rideID string) string {
	return fmt.Sprintf("ride:%s:request_state", rideID)
}

func GetRideWaveKey(rideID string) string {
	return fmt.Sprintf("ride:%s:wave", rideID)
}

func GetRidePendingDriversKey(rideID string) string {
	return fmt.Sprintf("ride:%s:pending_drivers", rideID)
}

func GetRideEventChannel(rideID string) string {
	return fmt.Sprintf("ride:%s:events", rideID)
}

func GetDriverEventChannel(driverID string) string {
	return fmt.Sprintf("driver:%s:events", driverID)
}

func GetOTPRideKey(rideID string) string {
	return fmt.Sprintf("ride:%s:otp", rideID)
}

func GetSOSAlertKey(alertID string) string {
	return fmt.Sprintf("sos:%s:alert", alertID)
}

// Cache helpers
func SetCache(key string, value interface{}, expiry time.Duration) error {
	return RedisClient.Set(Ctx, key, value, expiry).Err()
}

func GetCache(key string) (string, error) {
	return RedisClient.Get(Ctx, key).Result()
}

func DeleteCache(key string) error {
	return RedisClient.Del(Ctx, key).Err()
}

func SetCacheJSON(key string, value interface{}, expiry time.Duration) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return RedisClient.Set(Ctx, key, encoded, expiry).Err()
}

// Distributed lock helpers
func AcquireLock(key string, expiry time.Duration) (bool, error) {
	return RedisClient.SetNX(Ctx, key, "1", expiry).Result()
}

func ReleaseLock(key string) error {
	return RedisClient.Del(Ctx, key).Err()
}

// Geo helpers for driver location
func UpdateDriverGeoLocation(driverID string, lat, lng float64) error {
	key := "drivers:online:geo"
	return RedisClient.GeoAdd(Ctx, key, &redis.GeoLocation{
		Name:      driverID,
		Longitude: lng,
		Latitude:  lat,
	}).Err()
}

func RemoveDriverGeoLocation(driverID string) error {
	key := "drivers:online:geo"
	return RedisClient.ZRem(Ctx, key, driverID).Err()
}

func FindNearbyDrivers(lat, lng, radius float64) ([]redis.GeoLocation, error) {
	key := "drivers:online:geo"
	return RedisClient.GeoRadius(Ctx, key, lng, lat, &redis.GeoRadiusQuery{
		Radius:    radius,
		Unit:      "km",
		WithDist:  true,
		WithCoord: true,
		Count:     20,
		Sort:      "ASC",
	}).Result()
}

// Pub/Sub helpers for live tracking
func PublishLocationUpdate(channel string, data interface{}) error {
	return RedisClient.Publish(Ctx, channel, data).Err()
}

func SubscribeToChannel(channel string) *redis.PubSub {
	return RedisClient.Subscribe(Ctx, channel)
}
