package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/git-saj/go-media-control/handlers"
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

	// Set up router
	r := chi.NewRouter()
	r.Use(middleware.Logger)    // Log requests
	r.Use(middleware.Recoverer) // Recover from panics

	// Define routes
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})
	r.Get("/api/media", h.MediaHandler)

	// Start server
	logger.Info("Starting server", "port", cfg.Port)
	err = http.ListenAndServe(":"+cfg.Port, r)
	if err != nil {
		logger.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
