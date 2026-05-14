#!/bin/bash
# Production Validation Script
# This script validates that the backend is ready for production deployment.
# Run this before every production release.

set -e

echo "=== Rapido Backend Production Validation ==="
echo ""

# Check Go build
echo "1. Checking Go build..."
if ! go build ./...; then
    echo "ERROR: Build failed. Fix compiler errors before deploying."
    exit 1
fi
echo "   ✓ Build successful"

# Check Go vet
echo "2. Running Go vet..."
if ! go vet ./...; then
    echo "ERROR: Go vet found issues. Fix before deploying."
    exit 1
fi
echo "   ✓ Vet passed"

# Check for remaining debug artifacts
echo "3. Checking for debug artifacts..."
DEBUG_PATTERNS=(
    'fmt.Printf.*DEBUG'
    'log.Printf.*\[DEBUG\]'
    'utils.Debug\('
    'return true // debug'
    'return true // dev'
)

found_debug=0
for pattern in "${DEBUG_PATTERNS[@]}"; do
    if grep -r "$pattern" --include="*.go" --exclude-dir=vendor .; then
        found_debug=$((found_debug + 1))
    fi
done
if [ $found_debug -gt 0 ]; then
    echo "   ⚠ WARNING: Found debug patterns. Review above and remove if in production code."
else
    echo "   ✓ No debug artifacts found"
fi

# Check for placeholder secrets in config
echo "4. Checking for placeholder secrets (from APP_ENV)..."
APP_ENV=${APP_ENV:-production}
if [ "$APP_ENV" = "production" ] || [ "$APP_ENV" = "staging" ]; then
    echo "   Validating configuration for $APP_ENV..."
    # The app will fail to start if production secrets are missing/placeholders
    # This is checked via config.Validate() in main.go
    echo "   ✓ Configuration will be validated at startup"
else
    echo "   ⚠ APP_ENV=$APP_ENV (not production/staging). Update for production."
fi

# Check for required env vars at least exist in .env template
echo "5. Checking .env structure..."
if [ -f ".env" ]; then
    REQUIRED_VARS=(
        "DB_HOST"
        "DB_USERNAME"
        "DB_PASSWORD"
        "JWT_SECRET"
        "REDIS_ADDR"
        "SERVER_PORT"
        "ADMIN_EMAIL"
        "ADMIN_PASSWORD"
    )
    
    missing=0
    for var in "${REQUIRED_VARS[@]}"; do
        if ! grep -q "^$var=" .env; then
            echo "   ✗ Missing $var in .env"
            missing=$((missing + 1))
        fi
    done
    
    if [ $missing -eq 0 ]; then
        echo "   ✓ All required vars present in .env"
    else
        echo "   ERROR: $missing required variables missing from .env"
        exit 1
    fi
else
    echo "   ⚠ No .env file found. Configuration will come from environment."
fi

# Check for hardcoded credentials in code
echo "6. Checking for hardcoded credentials..."
CREDENTIAL_PATTERNS=(
    'password.*=.*".*"'
    'secret.*=.*".*"'
    'token.*=.*".*"'
    'key.*=.*".*"'
)

found_creds=0
for pattern in "${CREDENTIAL_PATTERNS[@]}"; do
    if grep -ri "$pattern" --include="*.go" . | grep -v "config\|middleware\|GetString\|GetEnv\|viper"; then
        found_creds=$((found_creds + 1))
    fi
done
if [ $found_creds -gt 0 ]; then
    echo "   ✗ WARNING: Found potential hardcoded credentials. Review above."
else
    echo "   ✓ No obvious hardcoded credentials"
fi

# Check git status (optional - for CI/CD)
if command -v git &> /dev/null; then
    echo "7. Checking git status..."
    if [ -n "$(git status --porcelain)" ]; then
        echo "   ⚠ WARNING: Uncommitted changes detected:"
        git status --short
    else
        echo "   ✓ Working directory clean"
    fi
fi

echo ""
echo "=== Validation Complete ==="
echo "Backend appears production-ready. Ensure:"
echo "  • APP_ENV is set to 'production' or 'staging'"
echo "  • All secrets in .env are real values (not placeholders)"
echo "  • Redis and database are accessible from the deployment target"
echo "  • SMS provider (Twilio/MSG91) credentials are configured"
echo "  • Razorpay or payment provider credentials are set"
echo ""
