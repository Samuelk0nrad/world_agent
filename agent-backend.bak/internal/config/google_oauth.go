package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func LoadGoogleOAuthConfig() GoogleOAuthConfig {
	cfg, err := NewViperFromEnv()
	if err != nil {
		return LoadGoogleOAuthConfigFromEnv(os.Getenv)
	}
	return LoadGoogleOAuthConfigFromViper(cfg)
}

func LoadGoogleOAuthConfigFromViper(cfg *viper.Viper) GoogleOAuthConfig {
	if cfg == nil {
		return LoadGoogleOAuthConfigFromEnv(os.Getenv)
	}
	return LoadGoogleOAuthConfigFromEnv(cfg.GetString)
}

func LoadGoogleOAuthConfigFromEnv(getEnv func(string) string) GoogleOAuthConfig {
	return GoogleOAuthConfig{
		ClientID:     strings.TrimSpace(getEnv("GOOGLE_CLIENT_ID")),
		ClientSecret: strings.TrimSpace(getEnv("GOOGLE_CLIENT_SECRET")),
		RedirectURL:  strings.TrimSpace(getEnv("GOOGLE_REDIRECT_URL")),
	}
}

func (c GoogleOAuthConfig) Validate() error {
	if strings.TrimSpace(c.ClientID) == "" {
		return fmt.Errorf("GOOGLE_CLIENT_ID is required")
	}
	if strings.TrimSpace(c.ClientSecret) == "" {
		return fmt.Errorf("GOOGLE_CLIENT_SECRET is required")
	}
	if strings.TrimSpace(c.RedirectURL) == "" {
		return fmt.Errorf("GOOGLE_REDIRECT_URL is required")
	}
	return nil
}
