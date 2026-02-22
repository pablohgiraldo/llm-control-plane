package routes

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/upb/llm-control-plane/backend/app"
	"github.com/upb/llm-control-plane/backend/handlers"
)

// SetupRoutes configures all application routes and middleware
func SetupRoutes(deps *app.Dependencies) http.Handler {
	r := chi.NewRouter()

	// Core middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS middleware
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "https://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoints
	r.Get("/healthz", handlers.HealthCheck(deps))
	r.Get("/readyz", handlers.ReadinessCheck(deps))

	// OAuth2 auth endpoints (Cognito)
	r.Route("/auth", func(r chi.Router) {
		r.Get("/login", handlers.AuthLoginHandler(deps))
		r.Get("/callback", handlers.AuthCallbackHandler(deps))
		r.Get("/logout", handlers.AuthLogoutHandler(deps))
	})
	// Cognito Hosted UI default callback path (also used by /auth/callback)
	r.Get("/oauth2/idpresponse", handlers.AuthCallbackHandler(deps))

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes
		r.Get("/status", handlers.StatusHandler(deps))

		// Inference endpoints (require authentication)
		r.Route("/inference", func(r chi.Router) {
			r.Use(deps.AuthMiddleware.RequireAuth)
			r.Use(deps.AuthMiddleware.ExtractTenant)
			r.Post("/chat", handlers.ChatCompletionHandler(deps))
			r.Get("/requests", handlers.ListInferenceRequestsHandler(deps))
			r.Get("/requests/{id}", handlers.GetInferenceRequestHandler(deps))
		})

		// Organization management
		r.Route("/organizations", func(r chi.Router) {
			r.Use(deps.AuthMiddleware.RequireAuth)
			r.Get("/", handlers.ListOrganizationsHandler(deps))
			r.Post("/", handlers.CreateOrganizationHandler(deps))
			r.Get("/{id}", handlers.GetOrganizationHandler(deps))
			r.Put("/{id}", handlers.UpdateOrganizationHandler(deps))
			r.Delete("/{id}", handlers.DeleteOrganizationHandler(deps))
		})

		// Application management
		r.Route("/applications", func(r chi.Router) {
			r.Use(deps.AuthMiddleware.RequireAuth)
			r.Get("/", handlers.ListApplicationsHandler(deps))
			r.Post("/", handlers.CreateApplicationHandler(deps))
			r.Get("/{id}", handlers.GetApplicationHandler(deps))
			r.Put("/{id}", handlers.UpdateApplicationHandler(deps))
			r.Delete("/{id}", handlers.DeleteApplicationHandler(deps))
		})

		// Policy management (require admin role)
		r.Route("/policies", func(r chi.Router) {
			r.Use(deps.AuthMiddleware.RequireAuth)
			r.Use(deps.AuthMiddleware.RequireRole("admin"))
			r.Get("/", handlers.ListPoliciesHandler(deps))
			r.Post("/", handlers.CreatePolicyHandler(deps))
			r.Get("/{id}", handlers.GetPolicyHandler(deps))
			r.Put("/{id}", handlers.UpdatePolicyHandler(deps))
			r.Delete("/{id}", handlers.DeletePolicyHandler(deps))
		})

		// Audit logs (require admin role)
		r.Route("/audit", func(r chi.Router) {
			r.Use(deps.AuthMiddleware.RequireAuth)
			r.Use(deps.AuthMiddleware.RequireRole("admin"))
			r.Get("/logs", handlers.ListAuditLogsHandler(deps))
			r.Get("/logs/{id}", handlers.GetAuditLogHandler(deps))
		})

		// User management
		r.Route("/users", func(r chi.Router) {
			r.Use(deps.AuthMiddleware.RequireAuth)
			r.Get("/", handlers.ListUsersHandler(deps))
			r.Get("/me", handlers.GetCurrentUserHandler(deps))
			r.Get("/{id}", handlers.GetUserHandler(deps))
			r.Put("/{id}", handlers.UpdateUserHandler(deps))
		})

		// Metrics and analytics
		r.Route("/metrics", func(r chi.Router) {
			r.Use(deps.AuthMiddleware.RequireAuth)
			r.Get("/inference", handlers.GetInferenceMetricsHandler(deps))
		})
	})

	// 404 handler
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"endpoint not found"}`))
	})

	return r
}
