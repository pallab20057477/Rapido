#!/bin/sh
# Loads common Docker secrets (if present) into environment variables
set -e

load_secret() {
  name="$1"
  file="/run/secrets/$name"
  if [ -f "$file" ]; then
    # Read and export the secret without trailing newline
    val=$(head -c 4096 "$file" | tr -d '\r')
    export "$name"="$val"
  fi
}

# Map secrets to env vars expected by the app
# Secrets are expected to be created as: DB_PASSWORD, JWT_SECRET, REDIS_PASSWORD, GRAFANA_PASSWORD
load_secret "DB_PASSWORD"
load_secret "JWT_SECRET"
load_secret "REDIS_PASSWORD"
load_secret "GRAFANA_PASSWORD"

# If the application expects specific env names for Redis/DB host/port, leave them as-is
# Execute the command given (or default to ./rapido-backend)
if [ "$#" -gt 0 ]; then
  exec "$@"
else
  exec ./rapido-backend
fi
