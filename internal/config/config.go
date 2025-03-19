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
