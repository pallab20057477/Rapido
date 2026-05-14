package services

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

// GeoPartitioner implements region-based sharding for 50K+ drivers
type GeoPartitioner struct {
	regions map[string]*RegionShard
	clients map[string]*redis.Client
	mu      sync.RWMutex
}

type RegionShard struct {
	Name          string
	GeoHashPrefix string
	RedisAddr     string
	MinLat        float64
	MaxLat        float64
	MinLng        float64
	MaxLng        float64
}

type DriverLocation struct {
	DriverID  string
	Lat       float64
	Lng       float64
	Distance  float64
	Timestamp int64
}

// indiaRegions returns region shard config, reading Redis addresses from env vars.
// Env vars: REDIS_SHARD_SOUTH, REDIS_SHARD_WEST, REDIS_SHARD_NORTH, REDIS_SHARD_EAST, REDIS_SHARD_CENTRAL
// Falls back to the primary REDIS_ADDR:REDIS_PORT when a shard env var is not set.
func indiaRegions() []RegionShard {
	primary := primaryRedisAddr()
	return []RegionShard{
		{Name: "south", GeoHashPrefix: "t", RedisAddr: shardAddr("REDIS_SHARD_SOUTH", primary), MinLat: 8.0, MaxLat: 20.0, MinLng: 72.0, MaxLng: 80.0},
		{Name: "west", GeoHashPrefix: "s", RedisAddr: shardAddr("REDIS_SHARD_WEST", primary), MinLat: 15.0, MaxLat: 25.0, MinLng: 68.0, MaxLng: 74.0},
		{Name: "north", GeoHashPrefix: "u", RedisAddr: shardAddr("REDIS_SHARD_NORTH", primary), MinLat: 25.0, MaxLat: 37.0, MinLng: 72.0, MaxLng: 90.0},
		{Name: "east", GeoHashPrefix: "w", RedisAddr: shardAddr("REDIS_SHARD_EAST", primary), MinLat: 17.0, MaxLat: 27.0, MinLng: 83.0, MaxLng: 93.0},
		{Name: "central", GeoHashPrefix: "v", RedisAddr: shardAddr("REDIS_SHARD_CENTRAL", primary), MinLat: 20.0, MaxLat: 26.0, MinLng: 74.0, MaxLng: 84.0},
	}
}

// primaryRedisAddr builds the primary Redis address from REDIS_ADDR / REDIS_PORT env vars.
func primaryRedisAddr() string {
	host := strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	if host == "" {
		host = "localhost"
	}
	port := strings.TrimSpace(os.Getenv("REDIS_PORT"))
	if port == "" {
		port = "6379"
	}
	// If host already contains a port (e.g. "localhost:6379") use it as-is
	if strings.Contains(host, ":") {
		return host
	}
	return host + ":" + port
}

// shardAddr returns the Redis address for a named shard, falling back to the primary.
func shardAddr(envKey, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	return fallback
}

func NewGeoPartitioner() *GeoPartitioner {
	gp := &GeoPartitioner{
		regions: make(map[string]*RegionShard),
		clients: make(map[string]*redis.Client),
	}

	for _, r := range indiaRegions() {
		rc := r // capture loop variable
		gp.regions[r.Name] = &rc
		gp.clients[r.Name] = redis.NewClient(&redis.Options{Addr: r.RedisAddr})
	}

	return gp
}

func (gp *GeoPartitioner) getShard(lat, lng float64) *RegionShard {
	regions := indiaRegions()
	for _, r := range regions {
		if lat >= r.MinLat && lat <= r.MaxLat && lng >= r.MinLng && lng <= r.MaxLng {
			return &r
		}
	}
	central := regions[4] // Default to central
	return &central
}

func (gp *GeoPartitioner) UpdateDriverLocation(driverID string, lat, lng float64) error {
	shard := gp.getShard(lat, lng)
	client := gp.clients[shard.Name]

	geoKey := fmt.Sprintf("drivers:online:%s", shard.Name)

	return client.GeoAdd(context.Background(), geoKey, &redis.GeoLocation{
		Name:      driverID,
		Latitude:  lat,
		Longitude: lng,
	}).Err()
}

func (gp *GeoPartitioner) FindNearbyDrivers(lat, lng, radiusKM float64) ([]DriverLocation, error) {
	shard := gp.getShard(lat, lng)

	// Query primary shard
	return gp.queryShard(shard, lat, lng, radiusKM)
}

func (gp *GeoPartitioner) queryShard(shard *RegionShard, lat, lng, radiusKM float64) ([]DriverLocation, error) {
	client := gp.clients[shard.Name]
	geoKey := fmt.Sprintf("drivers:online:%s", shard.Name)

	results, err := client.GeoRadius(context.Background(), geoKey, lng, lat, &redis.GeoRadiusQuery{
		Radius: radiusKM,
		Unit:   "km",
		Count:  100,
	}).Result()

	if err != nil {
		return nil, err
	}

	drivers := make([]DriverLocation, len(results))
	for i, loc := range results {
		drivers[i] = DriverLocation{
			DriverID: loc.Name,
			Lat:      loc.Latitude,
			Lng:      loc.Longitude,
			Distance: loc.Dist,
		}
	}

	sort.Slice(drivers, func(i, j int) bool {
		return drivers[i].Distance < drivers[j].Distance
	})

	return drivers, nil
}

var GeoPartitionerInstance *GeoPartitioner

func InitGeoPartitioner() {
	GeoPartitionerInstance = NewGeoPartitioner()
}
