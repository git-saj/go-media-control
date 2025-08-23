package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/git-saj/go-media-control/handlers"
	"github.com/git-saj/go-media-control/internal/auth"
	"github.com/git-saj/go-media-control/internal/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Set up logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Load Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize handlers with config values
	h := handlers.NewHandlers(logger, cfg)

	var authService *auth.AuthService
	var authHandlers *auth.AuthHandlers

	// Initialize authentication service only if not disabled
	if !cfg.DisableAuth {
		var err error
		authService, err = auth.NewAuthService(cfg, logger)
		if err != nil {
			logger.Error("Failed to initialize authentication service", "error", err)
			os.Exit(1)
		}
		authHandlers = auth.NewAuthHandlers(authService, logger)
		logger.Info("Authentication enabled")
	} else {
		logger.Info("Authentication disabled")
	}

	// Set up router
	r := chi.NewRouter()
	r.Use(middleware.Logger)    // Log requests
	r.Use(middleware.Recoverer) // Recover from panics

	// Handle routing based on base path
	if cfg.BasePath == "/" {
		// Root path - mount routes directly
		setupRoutes(r, cfg, h, authService, authHandlers)
	} else {
		// Subpath - mount under base path
		basePath := cfg.BasePath[:len(cfg.BasePath)-1] // Remove trailing slash
		r.Route(basePath, func(r chi.Router) {
			setupRoutes(r, cfg, h, authService, authHandlers)
		})
	}

	// Start server
	logger.Info("Starting server", "port", cfg.Port)
	err = http.ListenAndServe(":"+cfg.Port, r)
	if err != nil {
		logger.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}

// setupRoutes configures all application routes
func setupRoutes(r chi.Router, cfg *config.Config, h *handlers.Handlers, authService *auth.AuthService, authHandlers *auth.AuthHandlers) {
	// Public routes (no authentication required)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"go-media-control"}`))
	})

	if !cfg.DisableAuth {
		// Authentication routes (no auth required)
		r.Route("/auth", func(r chi.Router) {
			r.Get("/login", authHandlers.LoginHandler)
			r.Get("/callback", authHandlers.CallbackHandler)
			r.Get("/logout", authHandlers.LogoutHandler)
			r.Get("/logged-out", authHandlers.LoggedOutHandler)
			r.Get("/user", authHandlers.UserInfoHandler) // For debugging
		})

		// Protected routes (authentication required)
		r.Group(func(r chi.Router) {
			r.Use(authService.RequireAuth) // Apply authentication middleware

			// Serve static files with base path awareness
			staticPrefix := cfg.BasePath + "static/"
			r.Handle("/static/*", http.StripPrefix(staticPrefix, http.FileServer(http.Dir("static"))))

			// Define protected routes
			r.Get("/", h.HomeHandler)
			r.Get("/api/media", h.MediaHandler)
			r.Post("/api/send", h.SendHandler)
			r.Post("/search", h.SearchHandler)
			r.Get("/refresh", h.RefreshHandler)
		})
	} else {
		// No authentication - all routes are public
		// Serve static files with base path awareness
		staticPrefix := cfg.BasePath + "static/"
		r.Handle("/static/*", http.StripPrefix(staticPrefix, http.FileServer(http.Dir("static"))))

		// Define public routes
		r.Get("/", h.HomeHandler)
		r.Get("/api/media", h.MediaHandler)
		r.Post("/api/send", h.SendHandler)
		r.Post("/search", h.SearchHandler)
		r.Get("/refresh", h.RefreshHandler)
	}
}
