package config

import (
	"fmt"
	"os"
	"strings"
)

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

func LoadGoogleOAuthConfig() GoogleOAuthConfig {
	return LoadGoogleOAuthConfigFromEnv(os.Getenv)
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
