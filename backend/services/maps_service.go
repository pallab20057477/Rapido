package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"rapido-backend/config"
	"rapido-backend/utils"
	"time"
)

// GoogleMapsService handles Google Maps API integration
type GoogleMapsService struct {
	apiKey string
	client *http.Client
}

// NewGoogleMapsService creates a new Google Maps service
func NewGoogleMapsService() *GoogleMapsService {
	cfg := config.Get()
	return &GoogleMapsService{
		apiKey: cfg.Google.APIKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// DistanceMatrixResponse represents the distance matrix API response
type DistanceMatrixResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
	Rows         []struct {
		Elements []struct {
			Status   string `json:"status"`
			Duration struct {
				Value int    `json:"value"`
				Text  string `json:"text"`
			} `json:"duration"`
			Distance struct {
				Value int    `json:"value"`
				Text  string `json:"text"`
			} `json:"distance"`
		} `json:"elements"`
	} `json:"rows"`
}

// DirectionsResponse represents the directions API response
type DirectionsResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message,omitempty"`
	Routes       []struct {
		Summary string `json:"summary"`
		Legs    []struct {
			Distance struct {
				Value int    `json:"value"`
				Text  string `json:"text"`
			} `json:"distance"`
			Duration struct {
				Value int    `json:"value"`
				Text  string `json:"text"`
			} `json:"duration"`
			DurationInTraffic struct {
				Value int    `json:"value"`
				Text  string `json:"text"`
			} `json:"duration_in_traffic"`
			Steps []struct {
				Instructions string `json:"html_instructions"`
				Distance     struct {
					Value int    `json:"value"`
					Text  string `json:"text"`
				} `json:"distance"`
			} `json:"steps"`
		} `json:"legs"`
		OverviewPolyline struct {
			Points string `json:"points"`
		} `json:"overview_polyline"`
	} `json:"routes"`
}

// GeocodeResponse represents geocoding API response
type GeocodeResponse struct {
	Status  string `json:"status"`
	Results []struct {
		FormattedAddress string `json:"formatted_address"`
		Geometry         struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"geometry"`
	} `json:"results"`
}

// CalculateRoute calculates distance, duration and ETA using Google Maps Directions API
func (s *GoogleMapsService) CalculateRoute(originLat, originLng, destLat, destLng float64) (*RouteInfo, error) {
	if s.apiKey == "" {
		// Fallback to Haversine calculation
		return s.fallbackRoute(originLat, originLng, destLat, destLng)
	}

	origin := fmt.Sprintf("%f,%f", originLat, originLng)
	destination := fmt.Sprintf("%f,%f", destLat, destLng)

	u := fmt.Sprintf(
		"https://maps.googleapis.com/maps/api/directions/json?origin=%s&destination=%s&mode=driving&departure_time=now&traffic_model=best_guess&key=%s",
		url.QueryEscape(origin),
		url.QueryEscape(destination),
		s.apiKey,
	)

	resp, err := s.client.Get(u)
	if err != nil {
		return s.fallbackRoute(originLat, originLng, destLat, destLng)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var directions DirectionsResponse
	if err := json.Unmarshal(body, &directions); err != nil {
		return s.fallbackRoute(originLat, originLng, destLat, destLng)
	}

	if directions.Status != "OK" || len(directions.Routes) == 0 {
		return s.fallbackRoute(originLat, originLng, destLat, destLng)
	}

	route := directions.Routes[0]
	if len(route.Legs) == 0 {
		return s.fallbackRoute(originLat, originLng, destLat, destLng)
	}

	leg := route.Legs[0]

	// Use traffic-aware duration if available
	duration := leg.Duration.Value
	if leg.DurationInTraffic.Value > 0 {
		duration = leg.DurationInTraffic.Value
	}

	return &RouteInfo{
		DistanceKM:          float64(leg.Distance.Value) / 1000,
		DistanceText:        leg.Distance.Text,
		DurationSec:         duration,
		DurationText:        leg.Duration.Text,
		DurationTrafficText: leg.DurationInTraffic.Text,
		Polyline:            route.OverviewPolyline.Points,
		HasTrafficData:      leg.DurationInTraffic.Value > 0,
	}, nil
}

// CalculateDistanceMatrix calculates distance matrix for multiple origins/destinations
func (s *GoogleMapsService) CalculateDistanceMatrix(origins, destinations [][2]float64) (*DistanceMatrixResponse, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("no API key configured")
	}

	originStrs := make([]string, len(origins))
	for i, o := range origins {
		originStrs[i] = fmt.Sprintf("%f,%f", o[0], o[1])
	}

	destStrs := make([]string, len(destinations))
	for i, d := range destinations {
		destStrs[i] = fmt.Sprintf("%f,%f", d[0], d[1])
	}

	u := fmt.Sprintf(
		"https://maps.googleapis.com/maps/api/distancematrix/json?origins=%s&destinations=%s&mode=driving&departure_time=now&traffic_model=best_guess&key=%s",
		url.QueryEscape(joinPipe(originStrs)),
		url.QueryEscape(joinPipe(destStrs)),
		s.apiKey,
	)

	resp, err := s.client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var matrix DistanceMatrixResponse
	if err := json.Unmarshal(body, &matrix); err != nil {
		return nil, err
	}

	if matrix.Status != "OK" {
		return nil, fmt.Errorf("API error: %s - %s", matrix.Status, matrix.ErrorMessage)
	}

	return &matrix, nil
}

// GeocodeAddress converts address to coordinates
func (s *GoogleMapsService) GeocodeAddress(address string) (*GeocodeResult, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("no API key configured")
	}

	u := fmt.Sprintf(
		"https://maps.googleapis.com/maps/api/geocode/json?address=%s&key=%s",
		url.QueryEscape(address),
		s.apiKey,
	)

	resp, err := s.client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var geocode GeocodeResponse
	if err := json.Unmarshal(body, &geocode); err != nil {
		return nil, err
	}

	if geocode.Status != "OK" || len(geocode.Results) == 0 {
		return nil, fmt.Errorf("geocoding failed: %s", geocode.Status)
	}

	result := geocode.Results[0]
	return &GeocodeResult{
		Address: result.FormattedAddress,
		Lat:     result.Geometry.Location.Lat,
		Lng:     result.Geometry.Location.Lng,
	}, nil
}

// fallbackRoute uses Haversine formula as fallback
func (s *GoogleMapsService) fallbackRoute(originLat, originLng, destLat, destLng float64) (*RouteInfo, error) {
	distanceKM := utils.CalculateDistance(originLat, originLng, destLat, destLng)
	durationMin := utils.EstimateRideDuration(distanceKM, "bike", 1.0) // Default to bike for estimation
	durationSec := durationMin * 60

	return &RouteInfo{
		DistanceKM:     distanceKM,
		DistanceText:   fmt.Sprintf("%.1f km", distanceKM),
		DurationSec:    durationSec,
		DurationText:   fmt.Sprintf("%d mins", durationMin),
		HasTrafficData: false,
		IsFallback:     true,
	}, nil
}

// RouteInfo contains calculated route information
type RouteInfo struct {
	DistanceKM          float64 `json:"distance_km"`
	DistanceText        string  `json:"distance_text"`
	DurationSec         int     `json:"duration_sec"`
	DurationText        string  `json:"duration_text"`
	DurationTrafficText string  `json:"duration_traffic_text,omitempty"`
	Polyline            string  `json:"polyline,omitempty"`
	HasTrafficData      bool    `json:"has_traffic_data"`
	IsFallback          bool    `json:"is_fallback"`
}

// GeocodeResult represents a geocoded address
type GeocodeResult struct {
	Address string  `json:"address"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

// joinPipe joins strings with pipe separator
func joinPipe(strs []string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += "|"
		}
		result += s
	}
	return result
}

// GetInstance returns singleton instance
var mapsServiceInstance *GoogleMapsService

func GetMapsService() *GoogleMapsService {
	if mapsServiceInstance == nil {
		mapsServiceInstance = NewGoogleMapsService()
	}
	return mapsServiceInstance
}
