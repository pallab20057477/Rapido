package routes

import (
	"rapido-backend/controllers"
	"rapido-backend/database"
	"rapido-backend/middleware"
	"rapido-backend/websocket"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.Engine) {
	// Initialize controllers
	authController := controllers.NewAuthController()
	crmWebhookController := controllers.NewCRMWebhookController()
	driverController := controllers.NewDriverController()
	rideController := controllers.NewRideController()
	paymentController := controllers.NewPaymentController()
	adminController := controllers.NewAdminController()
	ecController := controllers.NewEmergencyContactController()
	ratingController := controllers.NewRatingController()
	scheduledRideController := controllers.NewScheduledRideController()
	supportTicketController := controllers.NewSupportTicketController()
	bulkAdminController := controllers.NewBulkAdminController()
	paymentMethodController := controllers.NewPaymentMethodController()
	notificationController := controllers.NewNotificationController()
	configController := controllers.NewConfigController()

	// Initialize middleware
	idempotencyMiddleware := middleware.NewIdempotencyMiddleware(database.RedisClient)

	// Apply API versioning to all API routes
	router.Use(middleware.VersioningMiddleware())

	// Public routes
	public := router.Group("/api/v1")
	{
		// Auth routes
		public.POST("/auth/otp/request", middleware.OTPRateLimit(), authController.RequestOTP)
		public.POST("/auth/otp/verify", middleware.StrictRateLimit(), authController.VerifyOTP)
		public.POST("/auth/refresh", authController.RefreshToken)
		public.POST("/auth/google", authController.GoogleLogin)
		public.POST("/auth/login", middleware.StrictRateLimit(), authController.PasswordLogin) // Password login
		public.POST("/webhooks/crm", middleware.WebhookSecurityMiddleware(), crmWebhookController.HandleWebhook)
		public.POST("/payments/webhook", middleware.WebhookSecurityMiddleware(), paymentController.HandlePaymentWebhook)
	}

	// Protected routes
	protected := router.Group("/api/v1")
	protected.Use(middleware.AuthMiddleware())
	{
		// Auth
		protected.POST("/auth/logout", authController.Logout)
		protected.GET("/auth/profile", authController.GetProfile)
		protected.PATCH("/auth/profile", authController.UpdateProfile)
		protected.POST("/auth/password/set", authController.SetPassword)       // Set password after OTP
		protected.POST("/auth/password/change", authController.ChangePassword) // Change password
		protected.GET("/auth/password/status", authController.HasPassword)     // Check if password is set
		// Emergency contacts - full CRUD
		protected.POST("/auth/emergency-contacts", ecController.AddEmergencyContact)
		protected.GET("/auth/emergency-contacts", ecController.GetEmergencyContacts)
		protected.PUT("/auth/emergency-contacts/:id", ecController.UpdateEmergencyContact)
		protected.DELETE("/auth/emergency-contacts/:id", ecController.RemoveEmergencyContact)

		// Rating routes
		protected.POST("/rides/:id/rate", ratingController.SubmitRating)
		protected.GET("/rides/:id/my-rating", ratingController.GetMyRideRating)
		protected.GET("/drivers/:id/reviews", ratingController.GetDriverReviews)
		protected.GET("/drivers/:id/rating-summary", ratingController.GetDriverRatingSummary)
		protected.POST("/ratings/:id/report", ratingController.ReportRating)

		// SOS endpoints
		protected.POST("/sos/trigger", ecController.TriggerSOS)
		protected.GET("/sos/history", ecController.GetMySOSHistory)

		// Support ticket endpoints
		protected.POST("/users/support/tickets", supportTicketController.CreateTicket)
		protected.GET("/users/support/tickets", supportTicketController.GetMyTickets)
		protected.GET("/users/support/tickets/:id", supportTicketController.GetTicketDetails)
		protected.POST("/users/support/tickets/:id/messages", supportTicketController.AddMessage)

		// Payment method routes
		protected.POST("/payments/methods/card", paymentMethodController.AddCard)
		protected.POST("/payments/methods/upi", paymentMethodController.AddUPI)
		protected.GET("/payments/methods", paymentMethodController.GetPaymentMethods)
		protected.DELETE("/payments/methods/:id", paymentMethodController.RemovePaymentMethod)
		protected.POST("/payments/methods/:id/default", paymentMethodController.SetDefaultPaymentMethod)

		// Driver registration (open to any authenticated user)
		protected.POST("/drivers/register", driverController.RegisterDriver)

		// Driver routes (RESTful - plural) - require driver role
		drivers := protected.Group("/drivers")
		drivers.Use(middleware.DriverMiddleware())
		{
			drivers.GET("/profile", middleware.StandardRateLimit(), driverController.GetDriverProfile)
			drivers.PATCH("/profile", middleware.StandardRateLimit(), driverController.UpdateDriverProfile)
			drivers.POST("/online", middleware.StandardRateLimit(), driverController.GoOnline)
			drivers.POST("/offline", middleware.StandardRateLimit(), driverController.GoOffline)
			drivers.POST("/location", middleware.StandardRateLimit(), driverController.UpdateLocation)
			drivers.GET("/earnings", driverController.GetDriverEarnings)
			drivers.GET("/stats", driverController.GetDriverStats)
		}

		// Ride estimation (rate limited, no auth required beyond JWT)
		protected.GET("/rides/estimate", middleware.StandardRateLimit(), rideController.EstimateFare)
		protected.GET("/drivers/nearby", middleware.StandardRateLimit(), rideController.GetNearbyDrivers)

		// Ride routes - static paths BEFORE dynamic :id to be explicit
		protected.POST("/rides", middleware.APIRateLimit(), idempotencyMiddleware.Middleware(), rideController.RequestRide)
		protected.GET("/rides/active", middleware.StandardRateLimit(), rideController.GetActiveRide)
		protected.GET("/rides/history", middleware.StandardRateLimit(), rideController.GetRideHistory)

		// Scheduled ride routes (static paths before :id)
		protected.POST("/rides/schedule", middleware.APIRateLimit(), idempotencyMiddleware.Middleware(), scheduledRideController.ScheduleRide)
		protected.GET("/rides/scheduled", middleware.StandardRateLimit(), scheduledRideController.GetScheduledRides)
		protected.GET("/rides/scheduled/:id", middleware.StandardRateLimit(), scheduledRideController.GetScheduledRideDetails)
		protected.PUT("/rides/scheduled/:id", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), scheduledRideController.UpdateScheduledRide)
		protected.POST("/rides/scheduled/:id/cancel", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), scheduledRideController.CancelScheduledRide)

		// Dynamic ride routes by :id
		protected.GET("/rides/:id", middleware.StandardRateLimit(), rideController.GetRide)
		protected.GET("/rides/:id/track", middleware.StandardRateLimit(), rideController.TrackRide)
		protected.GET("/rides/:id/eta", middleware.StandardRateLimit(), rideController.GetRideETA)
		protected.GET("/rides/:id/fare", middleware.StandardRateLimit(), rideController.GetFareBreakdown)

		// Rider actions
		protected.POST("/rides/:id/cancel", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), rideController.CancelRide)
		protected.POST("/rides/:id/apply-promo", middleware.StrictRateLimit(), rideController.ApplyPromoCode)
		protected.POST("/rides/:id/retry", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), rideController.RetryMatch)

		// Driver ride actions (with idempotency) - RESTful PATCH for status updates
		protected.POST("/rides/:id/accept", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), rideController.AcceptRide)
		protected.POST("/rides/:id/reject", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), rideController.RejectRide)
		protected.POST("/rides/:id/arrived", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), rideController.DriverArrived)
		protected.POST("/rides/:id/start", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), rideController.StartRide)
		protected.POST("/rides/:id/complete", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), rideController.CompleteRide)
		protected.PATCH("/rides/:id/status", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), rideController.UpdateRideStatus)
		protected.POST("/rides/:id/reassign", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), rideController.ReassignRide)

		// Cancellation reasons config
		protected.GET("/config/cancellation-reasons", configController.GetCancellationReasons)

		// Payment routes - /payments domain
		// Wallet & transactions
		protected.GET("/wallet", paymentController.GetWallet)
		protected.POST("/wallet/add-money", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), paymentController.AddMoneyToWallet)
		protected.GET("/transactions", paymentController.GetTransactionHistory)
		protected.POST("/withdrawals", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), paymentController.RequestWithdrawal)

		// Ride payments
		protected.POST("/payments/rides/:id/pay", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), paymentController.ProcessPayment)
		protected.POST("/payments/rides/:id/retry", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), paymentController.RetryPayment)
		protected.GET("/payments/rides/:id", paymentController.GetPaymentStatus)

		// Direct payment operations
		protected.POST("/payments/:id/refund", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), paymentController.RefundPayment)

		// Driver matching visibility and failure analysis (admin only, kept in protected group with AdminMiddleware)
		protected.GET("/rides/:id/match-status", middleware.AdminMiddleware(), rideController.GetMatchStatus)
		protected.GET("/rides/:id/failure-reason", middleware.AdminMiddleware(), rideController.GetFailureReason)
	}

	// Admin routes
	admin := router.Group("/api/v1/admin")
	admin.Use(middleware.AuthMiddleware(), middleware.AdminMiddleware())
	{
		// Driver verification (admin approval workflow) - static paths before dynamic :id
		admin.GET("/drivers/pending", middleware.StandardRateLimit(), adminController.ListPendingDrivers)                             // List unverified drivers
		admin.POST("/drivers/create", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), adminController.CreateDriver) // Manually create new driver
		admin.GET("/drivers/:id", middleware.StandardRateLimit(), adminController.GetDriverDetails)                                   // Get driver details
		admin.POST("/drivers/:id/verify", middleware.StrictRateLimit(), adminController.VerifyDriver)                                 // Approve/reject driver

		// Admin debug / emergency routes
		admin.GET("/debug/password", middleware.StrictRateLimit(), adminController.DebugUserPassword)         // Check user password status
		admin.POST("/reset-admin-password", middleware.StrictRateLimit(), adminController.ResetAdminPassword) // Force reset admin password from ENV

		// Admin dashboard
		admin.GET("/dashboard", middleware.StandardRateLimit(), adminController.GetDashboardStats)
		admin.GET("/rides", middleware.StandardRateLimit(), adminController.GetAllRides)
		admin.GET("/users", middleware.StandardRateLimit(), adminController.GetAllUsers)
		admin.GET("/drivers", middleware.StandardRateLimit(), adminController.GetAllDrivers)
		admin.GET("/payments", middleware.StandardRateLimit(), adminController.GetAllPayments)
		admin.GET("/withdrawals/pending", middleware.StandardRateLimit(), adminController.GetPendingWithdrawals)
		admin.POST("/withdrawals/process", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), adminController.ProcessWithdrawal)
		admin.POST("/surge-pricing", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), adminController.CreateSurgePricing)
		admin.DELETE("/surge-pricing/:id", middleware.StrictRateLimit(), adminController.RemoveSurgePricing)
		admin.POST("/promo-codes", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), adminController.CreatePromoCode)
		admin.GET("/reports", middleware.StandardRateLimit(), adminController.GetReports)

		// Ledger routes
		admin.GET("/ledger/accounts", middleware.StandardRateLimit(), adminController.GetLedgerAccounts)
		admin.GET("/ledger/entries", middleware.StandardRateLimit(), adminController.GetLedgerEntries)
		admin.POST("/ledger/audit-batch", middleware.StrictRateLimit(), adminController.AuditLedgerBatch)
		admin.GET("/ledger/account-balance", middleware.StandardRateLimit(), adminController.GetAccountBalance)

		// SOS Admin routes
		admin.GET("/sos/active", middleware.StandardRateLimit(), ecController.AdminGetActiveSOSEvents)
		admin.POST("/sos/:id/resolve", middleware.StrictRateLimit(), ecController.AdminResolveSOS)

		// Support ticket admin routes
		admin.GET("/support/tickets", middleware.StandardRateLimit(), supportTicketController.AdminGetAllTickets)
		admin.PUT("/support/tickets/:id", middleware.StrictRateLimit(), supportTicketController.AdminUpdateTicket)
		admin.POST("/support/tickets/:id/messages", middleware.StrictRateLimit(), supportTicketController.AdminAddMessage)

		// Admin-only config updates
		admin.PATCH("/config", middleware.StrictRateLimit(), adminController.UpdateAppConfig)

		// Bulk admin routes
		admin.POST("/bulk/verify-drivers", middleware.StrictRateLimit(), bulkAdminController.BulkVerifyDrivers)
		admin.POST("/bulk/notify", middleware.StrictRateLimit(), bulkAdminController.BulkNotify)
		admin.POST("/bulk/import-drivers", middleware.StrictRateLimit(), idempotencyMiddleware.Middleware(), bulkAdminController.BulkImportDrivers)
		admin.POST("/bulk/update-driver-status", middleware.StrictRateLimit(), bulkAdminController.BulkUpdateDriverStatus)
	}

	// Notification routes (user) - static paths before dynamic :id
	protected.GET("/notifications", notificationController.GetNotifications)
	protected.PATCH("/notifications/read-all", notificationController.MarkAllAsRead)
	protected.PATCH("/notifications/:id/read", notificationController.MarkAsRead)
	protected.DELETE("/notifications/:id", notificationController.DeleteNotification)

	// Unified Config API - role-based access on same resource
	// Public: returns public config (features, limits, timeouts)
	// Admin: returns full config including system status
	router.GET("/api/v1/config", middleware.OptionalAuthMiddleware(), configController.GetConfig)

	// WebSocket - unified endpoint with type query param for scalability
	// Supports horizontal scaling with Redis Pub/Sub
	router.GET("/ws", middleware.WebSocketAuthMiddleware(), middleware.WebSocketQueryValidator(), websocket.GetHandler().HandleWebSocket)
}
