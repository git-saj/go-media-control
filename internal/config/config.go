package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

func InitConfig() {
	viper.SetEnvPrefix("APP")
	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("MEDIA_SOURCE", "m3u")
	viper.SetDefault("M3U_URL", "")
	viper.SetDefault("XTREAMS_BASE_URL", "")
	viper.SetDefault("XTREAMS_USERNAME", "")
	viper.SetDefault("XTREAMS_PASSWORD", "")
	viper.SetDefault("DISCORD_WEBHOOK_URL", "")
	viper.SetDefault("PORT", "8080")

	// Explicitly bind environment variables
	viper.BindEnv("MEDIA_SOURCE")
	viper.BindEnv("M3U_URL")
	viper.BindEnv("XTREAMS_BASE_URL")
	viper.BindEnv("XTREAMS_USERNAME")
	viper.BindEnv("XTREAMS_PASSWORD")
	viper.BindEnv("DISCORD_WEBHOOK_URL")
	viper.BindEnv("PORT")

	// Force lowercase for media source to avoid case issues
	mediaSource := strings.ToLower(viper.GetString("MEDIA_SOURCE"))
	viper.Set("MEDIA_SOURCE", mediaSource)
}

func ValidateConfig() error {
	// Get media source and force to lowercase for consistency
	source := strings.ToLower(viper.GetString("MEDIA_SOURCE"))

	// Print config values for debugging
	fmt.Printf("Using media source: %s\n", source)

	switch source {
	case "m3u":
		m3uURL := viper.GetString("M3U_URL")
		if m3uURL == "" {
			return fmt.Errorf("M3U_URL is required for m3u source")
		}
		if _, err := url.Parse(m3uURL); err != nil {
			return fmt.Errorf("invalid M3U_URL: %v", err)
		}
	case "xtreams":
		baseURL := viper.GetString("XTREAMS_BASE_URL")
		username := viper.GetString("XTREAMS_USERNAME")
		password := viper.GetString("XTREAMS_PASSWORD")

		if baseURL == "" {
			return fmt.Errorf("XTREAMS_BASE_URL is required for xtreams source")
		}
		if username == "" {
			return fmt.Errorf("XTREAMS_USERNAME is required for xtreams source")
		}
		if password == "" {
			return fmt.Errorf("XTREAMS_PASSWORD is required for xtreams source")
		}
	default:
		return fmt.Errorf("MEDIA_SOURCE must be 'm3u' or 'xtreams', got '%s'", source)
	}

	if viper.GetString("DISCORD_WEBHOOK_URL") == "" {
		return fmt.Errorf("DISCORD_WEBHOOK_URL is required")
	}

	return nil
}
