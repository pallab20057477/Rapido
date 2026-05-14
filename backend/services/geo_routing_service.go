package services

import (
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// GeoRoutingService handles multi-city/region scaling
type GeoRoutingService struct {
	redis *redis.Client
}

// City represents a service region
type City struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"` // e.g., "BOM", "DEL", "BLR"
	Country     string `json:"country"`
	Timezone    string `json:"timezone"`
	BoundingBox struct {
		MinLat float64 `json:"min_lat"`
		MaxLat float64 `json:"max_lat"`
		MinLng float64 `json:"min_lng"`
		MaxLng float64 `json:"max_lng"`
	} `json:"bounding_box"`
	Center struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"center"`
	IsActive     bool   `json:"is_active"`
	ServerRegion string `json:"server_region"` // AWS region
}

func NewGeoRoutingService(redis *redis.Client) *GeoRoutingService {
	return &GeoRoutingService{redis: redis}
}

// GetCityFromCoordinates determines city from lat/lng
func (g *GeoRoutingService) GetCityFromCoordinates(lat, lng float64) (*City, error) {
	cities := g.getAllCities()

	for _, city := range cities {
		if lat >= city.BoundingBox.MinLat && lat <= city.BoundingBox.MaxLat &&
			lng >= city.BoundingBox.MinLng && lng <= city.BoundingBox.MaxLng {
			return &city, nil
		}
	}

	nearest := g.findNearestCity(lat, lng, cities)
	if nearest != nil {
		return nearest, nil
	}

	return nil, fmt.Errorf("no city found for coordinates: %f, %f", lat, lng)
}

// GetRoutingKey returns Redis key prefix for city-specific data
func (g *GeoRoutingService) GetRoutingKey(cityID string, dataType string) string {
	return fmt.Sprintf("%s:%s:%s", cityID, dataType, time.Now().Format("2006-01-02"))
}

// RouteRequest determines which server region should handle request
func (g *GeoRoutingService) RouteRequest(cityID string) map[string]interface{} {
	city := g.getCityByID(cityID)
	if city == nil {
		return map[string]interface{}{
			"routed_to": "default",
			"region":    "ap-south-1",
			"reason":    "city_not_found",
		}
	}

	return map[string]interface{}{
		"city_id":    city.ID,
		"city_name":  city.Name,
		"routed_to":  city.ServerRegion,
		"region":     city.ServerRegion,
		"latency_ms": 20,
	}
}

// GetActiveCities returns all active service cities
func (g *GeoRoutingService) GetActiveCities() []City {
	return g.getAllCities()
}

// CalculateDistance calculates distance between two points
func (g *GeoRoutingService) CalculateDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371
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

// Helper methods
func (g *GeoRoutingService) getAllCities() []City {
	return []City{
		{
			ID:       "city_mumbai",
			Name:     "Mumbai",
			Code:     "BOM",
			Country:  "India",
			Timezone: "Asia/Kolkata",
			BoundingBox: struct {
				MinLat float64 `json:"min_lat"`
				MaxLat float64 `json:"max_lat"`
				MinLng float64 `json:"min_lng"`
				MaxLng float64 `json:"max_lng"`
			}{
				MinLat: 18.89, MaxLat: 19.30,
				MinLng: 72.75, MaxLng: 72.95,
			},
			Center: struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			}{Lat: 19.0760, Lng: 72.8777},
			IsActive:     true,
			ServerRegion: "ap-south-1",
		},
		{
			ID:       "city_delhi",
			Name:     "Delhi",
			Code:     "DEL",
			Country:  "India",
			Timezone: "Asia/Kolkata",
			BoundingBox: struct {
				MinLat float64 `json:"min_lat"`
				MaxLat float64 `json:"max_lat"`
				MinLng float64 `json:"min_lng"`
				MaxLng float64 `json:"max_lng"`
			}{
				MinLat: 28.40, MaxLat: 28.88,
				MinLng: 76.84, MaxLng: 77.35,
			},
			Center: struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			}{Lat: 28.6139, Lng: 77.2090},
			IsActive:     true,
			ServerRegion: "ap-south-1",
		},
		{
			ID:       "city_bangalore",
			Name:     "Bangalore",
			Code:     "BLR",
			Country:  "India",
			Timezone: "Asia/Kolkata",
			BoundingBox: struct {
				MinLat float64 `json:"min_lat"`
				MaxLat float64 `json:"max_lat"`
				MinLng float64 `json:"min_lng"`
				MaxLng float64 `json:"max_lng"`
			}{
				MinLat: 12.85, MaxLat: 13.15,
				MinLng: 77.45, MaxLng: 77.75,
			},
			Center: struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			}{Lat: 12.9716, Lng: 77.5946},
			IsActive:     true,
			ServerRegion: "ap-south-1",
		},
	}
}

func (g *GeoRoutingService) getCityByID(cityID string) *City {
	cities := g.getAllCities()
	for _, city := range cities {
		if city.ID == cityID {
			return &city
		}
	}
	return nil
}

func (g *GeoRoutingService) findNearestCity(lat, lng float64, cities []City) *City {
	var nearest *City
	minDistance := math.MaxFloat64

	for i := range cities {
		dist := g.CalculateDistance(lat, lng, cities[i].Center.Lat, cities[i].Center.Lng)
		if dist < minDistance {
			minDistance = dist
			nearest = &cities[i]
		}
	}

	if minDistance <= 100 {
		return nearest
	}
	return nil
}

var GeoRoutingSvc *GeoRoutingService

func InitGeoRoutingService(redis *redis.Client) {
	GeoRoutingSvc = NewGeoRoutingService(redis)
}

func GetGeoRoutingService() *GeoRoutingService {
	return GeoRoutingSvc
}
