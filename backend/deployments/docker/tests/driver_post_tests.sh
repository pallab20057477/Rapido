#!/usr/bin/env bash
# Driver POST API test script
# Usage:
#   API_URL=http://localhost:8080/api/v1 DRIVER_JWT=<driver_jwt> ADMIN_JWT=<admin_jwt> ./driver_post_tests.sh
set -euo pipefail

API_URL=${API_URL:-http://localhost:8080/api/v1}
DRIVER_JWT=${DRIVER_JWT:-""}
ADMIN_JWT=${ADMIN_JWT:-""}

echo "Using API_URL=$API_URL"

# Helper
req() {
  local method=$1; shift
  local path=$1; shift
  local body=${1:-}
  local token=${2:-}

  echo "\n==> $method $path"
  if [ -n "$body" ]; then
    echo "Request body: $body"
  fi

  if [ -n "$token" ]; then
    curl -sS -X "$method" "$API_URL$path" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $token" \
      -d "$body" | jq . || true
  else
    curl -sS -X "$method" "$API_URL$path" \
      -H "Content-Type: application/json" \
      -d "$body" | jq . || true
  fi
}

# 1) Register Driver (requires authenticated user token)
register_body=$(cat <<'JSON'
{
  "license_number": "DL-TEST-1234",
  "license_image": "https://example.com/license.jpg",
  "license_expiry": "2030-12-31",
  "rc_number": "RC-TEST-1234",
  "rc_image": "https://example.com/rc.jpg",
  "aadhaar_number": "111122223333",
  "aadhaar_image": "https://example.com/aadhaar.jpg",
  "vehicle_type": "sedan",
  "vehicle_make": "Toyota",
  "vehicle_model": "Corolla",
  "vehicle_year": 2018,
  "vehicle_color": "white",
  "vehicle_number_plate": "XYZ-1234",
  "fuel_type": "petrol",
  "vehicle_image": "https://example.com/vehicle.jpg",
  "languages": ["en","hi"]
}
JSON
)
if [ -z "$DRIVER_JWT" ]; then
  echo "SKIP: RegisterDriver - no DRIVER_JWT provided"
else
  req POST "/drivers/register" "$register_body" "$DRIVER_JWT"
fi

# 2) Go Online
go_online_body='{"lat":12.9716, "lng":77.5946}'
if [ -z "$DRIVER_JWT" ]; then
  echo "SKIP: GoOnline - no DRIVER_JWT provided"
else
  req POST "/drivers/online" "$go_online_body" "$DRIVER_JWT"
fi

# 3) Update Location
update_loc_body='{"lat":12.9720, "lng":77.5950, "accuracy":5.0}'
if [ -z "$DRIVER_JWT" ]; then
  echo "SKIP: UpdateLocation - no DRIVER_JWT provided"
else
  req POST "/drivers/location" "$update_loc_body" "$DRIVER_JWT"
fi

# 4) Go Offline
if [ -z "$DRIVER_JWT" ]; then
  echo "SKIP: GoOffline - no DRIVER_JWT provided"
else
  req POST "/drivers/offline" "{}" "$DRIVER_JWT"
fi

# 5) Admin: Verify Driver (requires ADMIN_JWT)
verify_body='{"driver_id":"<driver_uuid_here>"}'
if [ -z "$ADMIN_JWT" ]; then
  echo "SKIP: VerifyDriver - no ADMIN_JWT provided"
else
  echo "NOTE: Replace <driver_uuid_here> with actual driver id returned by registration"
  req POST "/admin/drivers/verify" "$verify_body" "$ADMIN_JWT"
fi

echo "\nDriver API POST tests completed."
