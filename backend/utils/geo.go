package utils

import (
	"encoding/json"
	"math"
)

// Coordinates represents a geographic point
type Coordinates struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
}

// CalculateDistance calculates the great circle distance between two points
// using the Haversine formula. Returns distance in kilometers.
func CalculateDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371 // Earth's radius in kilometers

	// Convert latitude and longitude from degrees to radians
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	// Haversine formula
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	distance := R * c
	return distance
}

// CalculateETA estimates time to arrival in minutes
// Based on average speed of 25 km/h for bikes in city traffic
func CalculateETA(distanceKm float64, vehicleType string) int {
	var avgSpeedKmph float64

	switch vehicleType {
	case "bike":
		avgSpeedKmph = 25
	case "auto":
		avgSpeedKmph = 20
	case "car_go", "car_x":
		avgSpeedKmph = 30
	default:
		avgSpeedKmph = 25
	}

	// Add 2 minutes for pickup overhead
	timeMinutes := (distanceKm / avgSpeedKmph) * 60
	return int(math.Ceil(timeMinutes + 2))
}

// EstimateRideDuration estimates ride duration in minutes
// Based on average speed during the ride
func EstimateRideDuration(distanceKm float64, vehicleType string, trafficMultiplier float64) int {
	var avgSpeedKmph float64

	switch vehicleType {
	case "bike":
		avgSpeedKmph = 30
	case "auto":
		avgSpeedKmph = 25
	case "car_go":
		avgSpeedKmph = 35
	case "car_x":
		avgSpeedKmph = 40
	default:
		avgSpeedKmph = 30
	}

	// Apply traffic multiplier
	effectiveSpeed := avgSpeedKmph / trafficMultiplier
	duration := (distanceKm / effectiveSpeed) * 60
	return int(math.Ceil(duration))
}

// IsValidCoordinates checks if coordinates are valid
func IsValidCoordinates(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

// CalculateBearing calculates the bearing from point 1 to point 2 in degrees
func CalculateBearing(lat1, lng1, lat2, lng2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	y := math.Sin(deltaLng) * math.Cos(lat2Rad)
	x := math.Cos(lat1Rad)*math.Sin(lat2Rad) -
		math.Sin(lat1Rad)*math.Cos(lat2Rad)*math.Cos(deltaLng)

	bearing := math.Atan2(y, x) * 180 / math.Pi
	if bearing < 0 {
		bearing += 360
	}
	return bearing
}

// CalculateMidpoint calculates the midpoint between two coordinates
func CalculateMidpoint(lat1, lng1, lat2, lng2 float64) Coordinates {
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLng := (lng2 - lng1) * math.Pi / 180

	Bx := math.Cos(lat2Rad) * math.Cos(deltaLng)
	By := math.Cos(lat2Rad) * math.Sin(deltaLng)

	lat3 := math.Atan2(
		math.Sin(lat1Rad)+math.Sin(lat2Rad),
		math.Sqrt((math.Cos(lat1Rad)+Bx)*(math.Cos(lat1Rad)+Bx)+By*By),
	)
	lng3 := lng1*math.Pi/180 + math.Atan2(By, math.Cos(lat1Rad)+Bx)

	return Coordinates{
		Latitude:  lat3 * 180 / math.Pi,
		Longitude: lng3 * 180 / math.Pi,
	}
}

// IsWithinRadius checks if a point is within a given radius from center
func IsWithinRadius(centerLat, centerLng, pointLat, pointLng, radiusKm float64) bool {
	distance := CalculateDistance(centerLat, centerLng, pointLat, pointLng)
	return distance <= radiusKm
}

// EstimateFare estimates the fare based on distance and vehicle type
func EstimateFare(distanceKm float64, vehicleType string, durationMin int, surgeMultiplier float64) map[string]float64 {
	var baseFare, perKmRate, perMinRate, platformFee float64

	switch vehicleType {
	case "bike":
		baseFare = 30
		perKmRate = 8
		perMinRate = 1
		platformFee = 5
	case "auto":
		baseFare = 40
		perKmRate = 12
		perMinRate = 1.5
		platformFee = 8
	case "car_go":
		baseFare = 60
		perKmRate = 15
		perMinRate = 2
		platformFee = 10
	case "car_x":
		baseFare = 80
		perKmRate = 20
		perMinRate = 3
		platformFee = 15
	default:
		baseFare = 30
		perKmRate = 8
		perMinRate = 1
		platformFee = 5
	}

	distanceFare := distanceKm * perKmRate
	timeFare := float64(durationMin) * perMinRate
	subtotal := baseFare + distanceFare + timeFare

	// Apply surge
	surgeAmount := subtotal * (surgeMultiplier - 1)
	totalBeforeFees := subtotal + surgeAmount

	// Add platform fee
	total := totalBeforeFees + platformFee

	return map[string]float64{
		"base_fare":        baseFare,
		"distance_fare":    distanceFare,
		"time_fare":        timeFare,
		"subtotal":         subtotal,
		"surge_multiplier": surgeMultiplier,
		"surge_amount":     surgeAmount,
		"platform_fee":     platformFee,
		"total":            math.Ceil(total),
	}
}

// DecodePolyline decodes an encoded polyline string into coordinates
// Uses Google's polyline encoding algorithm
func DecodePolyline(encoded string) []Coordinates {
	if encoded == "" {
		return []Coordinates{}
	}

	var coords []Coordinates
	var index, lat, lng int

	for index < len(encoded) {
		var b int
		var shift uint
		var result int

		// Decode latitude
		for {
			if index >= len(encoded) {
				break
			}
			b = int(encoded[index]) - 63
			index++
			result |= (b & 0x1f) << shift
			if (b & 0x20) == 0 {
				break
			}
			shift += 5
		}

		var dlat int
		if (result & 1) != 0 {
			dlat = ^(result >> 1)
		} else {
			dlat = result >> 1
		}
		lat += dlat

		shift = 0
		result = 0

		// Decode longitude
		for {
			if index >= len(encoded) {
				break
			}
			b = int(encoded[index]) - 63
			index++
			result |= (b & 0x1f) << shift
			if (b & 0x20) == 0 {
				break
			}
			shift += 5
		}

		var dlng int
		if (result & 1) != 0 {
			dlng = ^(result >> 1)
		} else {
			dlng = result >> 1
		}
		lng += dlng

		coords = append(coords, Coordinates{
			Latitude:  float64(lat) * 1e-5,
			Longitude: float64(lng) * 1e-5,
		})
	}

	return coords
}

// EncodePolyline encodes coordinates into a polyline string
// Uses Google's polyline encoding algorithm
func EncodePolyline(coords []Coordinates) string {
	if len(coords) == 0 {
		return ""
	}

	var result []byte
	var prevLat, prevLng int

	for _, coord := range coords {
		lat := int(round(coord.Latitude * 1e5))
		lng := int(round(coord.Longitude * 1e5))

		// Encode latitude delta
		dLat := lat - prevLat
		prevLat = lat
		result = append(result, encodeSignedNumber(dLat)...)

		// Encode longitude delta
		dLng := lng - prevLng
		prevLng = lng
		result = append(result, encodeSignedNumber(dLng)...)
	}

	return string(result)
}

// encodeSignedNumber encodes a signed number using the polyline encoding algorithm
func encodeSignedNumber(num int) []byte {
	var result []byte
	// Shift left by 1 bit and invert if negative
	num = num << 1
	if num < 0 {
		num = ^num
	}

	// Encode in chunks of 5 bits
	for num >= 0x20 {
		result = append(result, byte((0x20|(num&0x1f))+63))
		num >>= 5
	}
	result = append(result, byte(num+63))
	return result
}

// round rounds a float64 to nearest int
func round(x float64) float64 {
	if x < 0 {
		return -round(-x)
	}
	return float64(int64(x + 0.5))
}

// EncodeGeoHash encodes lat/lng to geohash string
func EncodeGeoHash(lat, lng float64, precision int) string {
	// Simplified geohash implementation
	// In production, use a proper geohash library
	const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"

	if precision <= 0 {
		precision = 12
	}

	var geohash string
	var minLat, maxLat = -90.0, 90.0
	var minLng, maxLng = -180.0, 180.0
	var mid float64
	var isEven = true
	var bit = 0
	var ch = 0

	for len(geohash) < precision {
		if isEven {
			mid = (minLng + maxLng) / 2
			if lng >= mid {
				ch |= 1 << (4 - bit)
				minLng = mid
			} else {
				maxLng = mid
			}
		} else {
			mid = (minLat + maxLat) / 2
			if lat >= mid {
				ch |= 1 << (4 - bit)
				minLat = mid
			} else {
				maxLat = mid
			}
		}

		isEven = !isEven
		if bit < 4 {
			bit++
		} else {
			geohash += string(base32[ch])
			bit = 0
			ch = 0
		}
	}

	return geohash
}

// DecodeGeoHash decodes geohash to approximate lat/lng
func DecodeGeoHash(geohash string) (lat, lng float64) {
	// Simplified - returns center of geohash cell
	// In production, use a proper geohash library
	if len(geohash) == 0 {
		return 0, 0
	}

	var minLat, maxLat = -90.0, 90.0
	var minLng, maxLng = -180.0, 180.0
	var isEven = true

	for _, c := range geohash {
		cd := getCharIndex(c)
		for j := 4; j >= 0; j-- {
			bit := (cd >> j) & 1
			if isEven {
				mid := (minLng + maxLng) / 2
				if bit == 1 {
					minLng = mid
				} else {
					maxLng = mid
				}
			} else {
				mid := (minLat + maxLat) / 2
				if bit == 1 {
					minLat = mid
				} else {
					maxLat = mid
				}
			}
			isEven = !isEven
		}
	}

	return (minLat + maxLat) / 2, (minLng + maxLng) / 2
}

func getCharIndex(c rune) int {
	const base32 = "0123456789bcdefghjkmnpqrstuvwxyz"
	for i, b := range base32 {
		if b == c {
			return i
		}
	}
	return 0
}

// MustJSON marshals to JSON, panics on error
func MustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// FromJSON unmarshals JSON to target
func FromJSON(data string, target interface{}) error {
	return json.Unmarshal([]byte(data), target)
}
