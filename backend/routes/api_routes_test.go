package routes

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type routeSpec struct {
	method string
	path   string
}

func TestAPIRouteRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	healthRoutesRegistered = false

	router := gin.New()
	SetupHealthRoutes(router)
	SetupRoutes(router)

	actual := make(map[string]struct{})
	for _, route := range router.Routes() {
		actual[route.Method+" "+route.Path] = struct{}{}
	}

	expected := []routeSpec{
		{"GET", "/health"},
		{"GET", "/health/detailed"},
		{"GET", "/ready"},
		{"GET", "/live"},
		{"GET", "/metrics"},
		{"POST", "/api/v1/auth/otp/request"},
		{"POST", "/api/v1/auth/otp/verify"},
		{"POST", "/api/v1/auth/refresh"},
		{"POST", "/api/v1/auth/google"},
		{"POST", "/api/v1/auth/login"},
		{"POST", "/api/v1/webhooks/crm"},
		{"POST", "/api/v1/payments/webhook"},
		{"POST", "/api/v1/auth/logout"},
		{"GET", "/api/v1/auth/profile"},
		{"PATCH", "/api/v1/auth/profile"},
		{"POST", "/api/v1/auth/password/set"},
		{"POST", "/api/v1/auth/password/change"},
		{"GET", "/api/v1/auth/password/status"},
		{"POST", "/api/v1/auth/emergency-contacts"},
		{"GET", "/api/v1/auth/emergency-contacts"},
		{"PUT", "/api/v1/auth/emergency-contacts/:id"},
		{"DELETE", "/api/v1/auth/emergency-contacts/:id"},
		{"POST", "/api/v1/rides/:id/rate"},
		{"GET", "/api/v1/rides/:id/my-rating"},
		{"GET", "/api/v1/drivers/:id/reviews"},
		{"GET", "/api/v1/drivers/:id/rating-summary"},
		{"POST", "/api/v1/ratings/:id/report"},
		{"POST", "/api/v1/sos/trigger"},
		{"GET", "/api/v1/sos/history"},
		{"POST", "/api/v1/users/support/tickets"},
		{"GET", "/api/v1/users/support/tickets"},
		{"GET", "/api/v1/users/support/tickets/:id"},
		{"POST", "/api/v1/users/support/tickets/:id/messages"},
		{"POST", "/api/v1/payments/methods/card"},
		{"POST", "/api/v1/payments/methods/upi"},
		{"GET", "/api/v1/payments/methods"},
		{"DELETE", "/api/v1/payments/methods/:id"},
		{"POST", "/api/v1/payments/methods/:id/default"},
		{"POST", "/api/v1/drivers/register"},
		{"GET", "/api/v1/drivers/profile"},
		{"PATCH", "/api/v1/drivers/profile"},
		{"POST", "/api/v1/drivers/online"},
		{"POST", "/api/v1/drivers/offline"},
		{"POST", "/api/v1/drivers/location"},
		{"GET", "/api/v1/drivers/earnings"},
		{"GET", "/api/v1/drivers/stats"},
		{"GET", "/api/v1/rides/estimate"},
		{"GET", "/api/v1/drivers/nearby"},
		{"POST", "/api/v1/rides"},
		{"GET", "/api/v1/rides/active"},
		{"GET", "/api/v1/rides/history"},
		{"POST", "/api/v1/rides/schedule"},
		{"GET", "/api/v1/rides/scheduled"},
		{"GET", "/api/v1/rides/scheduled/:id"},
		{"PUT", "/api/v1/rides/scheduled/:id"},
		{"POST", "/api/v1/rides/scheduled/:id/cancel"},
		{"GET", "/api/v1/rides/:id"},
		{"GET", "/api/v1/rides/:id/track"},
		{"GET", "/api/v1/rides/:id/eta"},
		{"GET", "/api/v1/rides/:id/fare"},
		{"POST", "/api/v1/rides/:id/cancel"},
		{"POST", "/api/v1/rides/:id/apply-promo"},
		{"POST", "/api/v1/rides/:id/retry"},
		{"POST", "/api/v1/rides/:id/accept"},
		{"POST", "/api/v1/rides/:id/reject"},
		{"POST", "/api/v1/rides/:id/arrived"},
		{"POST", "/api/v1/rides/:id/start"},
		{"POST", "/api/v1/rides/:id/complete"},
		{"PATCH", "/api/v1/rides/:id/status"},
		{"POST", "/api/v1/rides/:id/reassign"},
		{"GET", "/api/v1/config/cancellation-reasons"},
		{"GET", "/api/v1/wallet"},
		{"POST", "/api/v1/wallet/add-money"},
		{"GET", "/api/v1/transactions"},
		{"POST", "/api/v1/withdrawals"},
		{"POST", "/api/v1/payments/rides/:id/pay"},
		{"POST", "/api/v1/payments/rides/:id/retry"},
		{"GET", "/api/v1/payments/rides/:id"},
		{"POST", "/api/v1/payments/:id/refund"},
		{"GET", "/api/v1/rides/:id/match-status"},
		{"GET", "/api/v1/rides/:id/failure-reason"},
		{"GET", "/api/v1/admin/drivers/pending"},
		{"POST", "/api/v1/admin/drivers/create"},
		{"GET", "/api/v1/admin/drivers/:id"},
		{"POST", "/api/v1/admin/drivers/:id/verify"},
		{"GET", "/api/v1/admin/debug/password"},
		{"POST", "/api/v1/admin/reset-admin-password"},
		{"GET", "/api/v1/admin/dashboard"},
		{"GET", "/api/v1/admin/rides"},
		{"GET", "/api/v1/admin/users"},
		{"GET", "/api/v1/admin/drivers"},
		{"GET", "/api/v1/admin/payments"},
		{"GET", "/api/v1/admin/withdrawals/pending"},
		{"POST", "/api/v1/admin/withdrawals/process"},
		{"POST", "/api/v1/admin/surge-pricing"},
		{"DELETE", "/api/v1/admin/surge-pricing/:id"},
		{"POST", "/api/v1/admin/promo-codes"},
		{"GET", "/api/v1/admin/reports"},
		{"GET", "/api/v1/admin/ledger/accounts"},
		{"GET", "/api/v1/admin/ledger/entries"},
		{"POST", "/api/v1/admin/ledger/audit-batch"},
		{"GET", "/api/v1/admin/ledger/account-balance"},
		{"GET", "/api/v1/admin/sos/active"},
		{"POST", "/api/v1/admin/sos/:id/resolve"},
		{"GET", "/api/v1/admin/support/tickets"},
		{"PUT", "/api/v1/admin/support/tickets/:id"},
		{"POST", "/api/v1/admin/support/tickets/:id/messages"},
		{"PATCH", "/api/v1/admin/config"},
		{"POST", "/api/v1/admin/bulk/verify-drivers"},
		{"POST", "/api/v1/admin/bulk/notify"},
		{"POST", "/api/v1/admin/bulk/import-drivers"},
		{"POST", "/api/v1/admin/bulk/update-driver-status"},
		{"GET", "/api/v1/notifications"},
		{"PATCH", "/api/v1/notifications/read-all"},
		{"PATCH", "/api/v1/notifications/:id/read"},
		{"DELETE", "/api/v1/notifications/:id"},
		{"GET", "/api/v1/config"},
		{"GET", "/ws"},
	}

	missing := make([]string, 0)
	for _, spec := range expected {
		key := spec.method + " " + spec.path
		if _, ok := actual[key]; !ok {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("missing routes: %s", strings.Join(missing, ", "))
	}
}

func TestHealthRoutesSmoke(t *testing.T) {
	gin.SetMode(gin.TestMode)
	healthRoutesRegistered = false

	router := gin.New()
	SetupHealthRoutes(router)

	cases := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/health", http.StatusOK},
		{http.MethodGet, "/live", http.StatusOK},
	}

	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req, err := http.NewRequest(tc.method, tc.path, nil)
		if err != nil {
			t.Fatalf("create request %s %s: %v", tc.method, tc.path, err)
		}

		router.ServeHTTP(rec, req)

		if rec.Code != tc.want {
			t.Fatalf("%s %s: want status %d got %d body=%s", tc.method, tc.path, tc.want, rec.Code, rec.Body.String())
		}
	}
}
