package config

import (
	"fmt"
	"os"
)

// Config holds the configuration for the application
type Config struct {
	XtreamBaseURL  string
	XtreamUsername string
	XtreamPassword string
	DiscordWebhook string
	CommandPrefix  string
	Port           string
	// Authentik OIDC configuration
	AuthentikURL  string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	SessionSecret string
}

// LoadConfig reads the environment variables and returns a Config struct
func LoadConfig() (*Config, error) {
	cfg := &Config{
		XtreamBaseURL:  os.Getenv("XTREAM_BASEURL"),
		XtreamUsername: os.Getenv("XTREAM_USERNAME"),
		XtreamPassword: os.Getenv("XTREAM_PASSWORD"),
		DiscordWebhook: os.Getenv("DISCORD_WEBHOOK"),
		CommandPrefix:  os.Getenv("COMMAND_PREFIX"),
		Port:           os.Getenv("PORT"),
		// Authentik OIDC configuration
		AuthentikURL:  os.Getenv("AUTHENTIK_URL"),
		ClientID:      os.Getenv("AUTHENTIK_CLIENT_ID"),
		ClientSecret:  os.Getenv("AUTHENTIK_CLIENT_SECRET"),
		RedirectURL:   os.Getenv("AUTHENTIK_REDIRECT_URL"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
	}

	// Validate required fields
	if cfg.XtreamBaseURL == "" {
		return nil, fmt.Errorf("XTREAM_BASEURL is required")
	}
	if cfg.XtreamUsername == "" {
		return nil, fmt.Errorf("XTREAM_USERNAME is required")
	}
	if cfg.XtreamPassword == "" {
		return nil, fmt.Errorf("XTREAM_PASSWORD is required")
	}
	if cfg.DiscordWebhook == "" {
		return nil, fmt.Errorf("DISCORD_WEBHOOK is required")
	}
	if cfg.AuthentikURL == "" {
		return nil, fmt.Errorf("AUTHENTIK_URL is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("AUTHENTIK_CLIENT_ID is required")
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("AUTHENTIK_CLIENT_SECRET is required")
	}
	if cfg.RedirectURL == "" {
		return nil, fmt.Errorf("AUTHENTIK_REDIRECT_URL is required")
	}
	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required")
	}

	// Set a default command prefix if not provided
	if cfg.CommandPrefix == "" {
		cfg.CommandPrefix = "!"
	}

	// Set the default port if not provided
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	return cfg, nil
}
