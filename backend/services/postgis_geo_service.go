package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// PostGISGeoService handles geospatial queries using PostgreSQL + PostGIS
type PostGISGeoService struct {
	db    *gorm.DB
	redis *redis.Client
}

// GeoDriverLocation represents a driver's location with PostGIS support
type GeoDriverLocation struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	DriverID    string    `gorm:"index" json:"driver_id"`
	Lat         float64   `json:"lat"`
	Lng         float64   `json:"lng"`
	Geom        string    `gorm:"-" json:"-"`          // ST_Point
	Status      string    `gorm:"index" json:"status"` // online, offline, busy
	VehicleType string    `json:"vehicle_type"`
	CityID      string    `gorm:"index" json:"city_id"`
	Accuracy    float64   `json:"accuracy"` // GPS accuracy in meters
	Heading     float64   `json:"heading"`  // Direction in degrees
	Speed       float64   `json:"speed"`    // km/h
	UpdatedAt   time.Time `json:"updated_at"`
	IsDeleted   bool      `json:"is_deleted"`
}

type NearbyDriverResult struct {
	DriverID   string  `json:"driver_id"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	Distance   float64 `json:"distance_km"`
	ETA        int     `json:"eta_seconds"`
	Rating     float64 `json:"rating"`
	TotalRides int     `json:"total_rides"`
}

func NewPostGISGeoService(db *gorm.DB, redis *redis.Client) *PostGISGeoService {
	return &PostGISGeoService{
		db:    db,
		redis: redis,
	}
}

// CreateDriverLocationTable creates PostGIS-enabled table
func (p *PostGISGeoService) CreateDriverLocationTable() error {
	// Enable PostGIS extension
	p.db.Exec("CREATE EXTENSION IF NOT EXISTS postgis;")

	// Create table with geometry column
	schema := `
	CREATE TABLE IF NOT EXISTS driver_locations_postgis (
		id VARCHAR(255) PRIMARY KEY,
		driver_id VARCHAR(255) NOT NULL,
		lat DOUBLE PRECISION,
		lng DOUBLE PRECISION,
		geom GEOMETRY(Point, 4326),
		status VARCHAR(50),
		vehicle_type VARCHAR(50),
		city_id VARCHAR(50),
		accuracy DOUBLE PRECISION DEFAULT 0,
		heading DOUBLE PRECISION DEFAULT 0,
		speed DOUBLE PRECISION DEFAULT 0,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		is_deleted BOOLEAN DEFAULT FALSE
	);

	CREATE INDEX IF NOT EXISTS idx_driver_geom_postgis 
	ON driver_locations_postgis USING GIST(geom);

	CREATE INDEX IF NOT EXISTS idx_driver_status_postgis 
	ON driver_locations_postgis(status);

	CREATE INDEX IF NOT EXISTS idx_driver_city_postgis 
	ON driver_locations_postgis(city_id);

	CREATE INDEX IF NOT EXISTS idx_driver_vehicle_postgis 
	ON driver_locations_postgis(vehicle_type, status)
	WHERE status = 'online';
	`

	return p.db.Exec(schema).Error
}

// UpdateDriverLocation updates driver location with PostGIS geometry
func (p *PostGISGeoService) UpdateDriverLocation(driverID string, lat, lng float64, metadata map[string]interface{}) error {
	// Validate coordinates
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return fmt.Errorf("invalid coordinates: lat=%f, lng=%f", lat, lng)
	}

	// Check Redis first for fast path
	cacheKey := fmt.Sprintf("driver:loc:%s", driverID)

	// Build update query
	query := `
		INSERT INTO driver_locations_postgis (id, driver_id, lat, lng, geom, status, vehicle_type, city_id, accuracy, heading, speed, updated_at)
		VALUES (
			?, ?, ?, ?, 
			ST_SetSRID(ST_MakePoint(?, ?), 4326),
			COALESCE(?, 'online'),
			COALESCE(?, 'bike'),
			COALESCE(?, 'city_default'),
			COALESCE(?, 0),
			COALESCE(?, 0),
			COALESCE(?, 0),
			NOW()
		)
		ON CONFLICT (id) DO UPDATE SET
			lat = EXCLUDED.lat,
			lng = EXCLUDED.lng,
			geom = EXCLUDED.geom,
			status = EXCLUDED.status,
			accuracy = EXCLUDED.accuracy,
			heading = EXCLUDED.heading,
			speed = EXCLUDED.speed,
			updated_at = EXCLUDED.updated_at;
	`

	id := driverID
	status := metadata["status"]
	vehicleType := metadata["vehicle_type"]
	cityID := metadata["city_id"]
	accuracy := metadata["accuracy"]
	heading := metadata["heading"]
	speed := metadata["speed"]

	err := p.db.Exec(query, id, driverID, lat, lng, lng, lat, status, vehicleType, cityID, accuracy, heading, speed).Error
	if err != nil {
		return err
	}

	// Also update Redis for fast reads
	locationData := map[string]interface{}{
		"lat":        lat,
		"lng":        lng,
		"updated_at": time.Now().Unix(),
		"status":     status,
	}
	if data, err := json.Marshal(locationData); err == nil {
		p.redis.Set(context.Background(), cacheKey, data, 5*time.Minute)
	}

	return nil
}

// FindNearbyDrivers finds drivers within radius using PostGIS
func (p *PostGISGeoService) FindNearbyDrivers(lat, lng float64, radiusKm float64, vehicleType string, limit int) ([]NearbyDriverResult, error) {
	// Try Redis GEO first for speed
	redisKey := fmt.Sprintf("geo:city:%s:%s", "default", vehicleType)

	// Query Redis GEO radius
	locations, err := p.redis.GeoRadius(context.Background(), redisKey, lng, lat, &redis.GeoRadiusQuery{
		Radius:    radiusKm,
		Unit:      "km",
		WithDist:  true,
		WithCoord: true,
		Count:     limit,
		Sort:      "ASC",
	}).Result()

	if err == nil && len(locations) > 0 {
		// Found in Redis
		var results []NearbyDriverResult
		for _, loc := range locations {
			driverID := loc.Name
			results = append(results, NearbyDriverResult{
				DriverID: driverID,
				Lat:      loc.Latitude,
				Lng:      loc.Longitude,
				Distance: loc.Dist,
				ETA:      int(loc.Dist * 2 * 60), // Rough estimate: 2 min per km
			})
		}
		return results, nil
	}

	// Fallback to PostGIS
	query := `
		SELECT 
			driver_id,
			lat,
			lng,
			ST_Distance(
				geom::geography,
				ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography
			) / 1000 AS distance
		FROM driver_locations_postgis
		WHERE status = 'online'
		AND is_deleted = FALSE
		AND vehicle_type = ?
		AND ST_DWithin(
			geom::geography,
			ST_SetSRID(ST_MakePoint(?, ?), 4326)::geography,
			? * 1000  -- Convert km to meters
		)
		ORDER BY geom <-> ST_SetSRID(ST_MakePoint(?, ?), 4326)
		LIMIT ?;
	`

	var results []NearbyDriverResult
	err = p.db.Raw(query, lng, lat, vehicleType, lng, lat, radiusKm, lng, lat, limit).Scan(&results).Error
	if err != nil {
		return nil, err
	}

	// Calculate ETA based on distance
	for i := range results {
		results[i].ETA = int(results[i].Distance * 2 * 60) // 2 min per km rough estimate
	}

	return results, nil
}

// FindNearestDriver finds the single nearest driver
func (p *PostGISGeoService) FindNearestDriver(lat, lng float64, vehicleType string) (*NearbyDriverResult, error) {
	drivers, err := p.FindNearbyDrivers(lat, lng, 5, vehicleType, 1)
	if err != nil {
		return nil, err
	}
	if len(drivers) == 0 {
		return nil, fmt.Errorf("no drivers available")
	}
	return &drivers[0], nil
}

// GetDriversWithinPolygon finds drivers within a polygon (for surge areas)
func (p *PostGISGeoService) GetDriversWithinPolygon(polygonWKT string, status string) ([]GeoDriverLocation, error) {
	query := `
		SELECT driver_id, lat, lng, status, vehicle_type, city_id, updated_at
		FROM driver_locations_postgis
		WHERE is_deleted = FALSE
		AND status = ?
		AND ST_Within(
			geom,
			ST_GeomFromText(?, 4326)
		);
	`

	var drivers []GeoDriverLocation
	err := p.db.Raw(query, status, polygonWKT).Scan(&drivers).Error
	return drivers, err
}

// BatchUpdateLocations updates multiple driver locations efficiently
func (p *PostGISGeoService) BatchUpdateLocations(locations []map[string]interface{}) error {
	tx := p.db.Begin()

	for _, loc := range locations {
		driverID := loc["driver_id"].(string)
		lat := loc["lat"].(float64)
		lng := loc["lng"].(float64)

		err := p.UpdateDriverLocation(driverID, lat, lng, loc)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// OptimizeLocationUpdate optimizes GPS updates (delta encoding)
func (p *PostGISGeoService) OptimizeLocationUpdate(driverID string, newLat, newLng float64) bool {
	// Get last location from Redis
	cacheKey := fmt.Sprintf("driver:lastloc:%s", driverID)

	lastLoc, err := p.redis.Get(context.Background(), cacheKey).Result()
	if err != nil {
		// No previous location, store and return true (send update)
		p.redis.Set(context.Background(), cacheKey, fmt.Sprintf("%f,%f", newLat, newLng), 1*time.Hour)
		return true
	}

	// Parse last location
	var lastLat, lastLng float64
	fmt.Sscanf(lastLoc, "%f,%f", &lastLat, &lastLng)

	// Calculate distance
	dist := calcGeoDistance(lastLat, lastLng, newLat, newLng)

	// Only update if moved more than 50 meters
	if dist < 0.05 { // 50 meters in km
		return false // Skip update (within threshold)
	}

	// Update last location
	p.redis.Set(context.Background(), cacheKey, fmt.Sprintf("%f,%f", newLat, newLng), 1*time.Hour)
	return true
}

// calcGeoDistance calculates distance between two points
func calcGeoDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371 // Earth radius in km

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// PostGISConfig returns configuration
func GetPostGISGeoConfig() map[string]interface{} {
	return map[string]interface{}{
		"extension":            "postgis",
		"srid":                 4326,
		"index_type":           "GIST",
		"distance_unit":        "kilometers",
		"update_threshold":     50, // meters
		"nearby_search_radius": 5,  // km
		"max_search_radius":    20, // km
		"batch_size":           1000,
		"redis_ttl":            300, // seconds
	}
}

var PostGISGeoSvc *PostGISGeoService

func InitPostGISGeoService(db *gorm.DB, redis *redis.Client) {
	PostGISGeoSvc = NewPostGISGeoService(db, redis)
	PostGISGeoSvc.CreateDriverLocationTable()
}

func GetPostGISGeoService() *PostGISGeoService {
	return PostGISGeoSvc
}
