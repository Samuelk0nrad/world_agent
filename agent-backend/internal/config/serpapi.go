package config

import (
	"fmt"
	"os"
	"strings"
)

const DefaultSerpAPIEngine = "google"

type SerpAPIConfig struct {
	APIKey string
	Engine string
}

func LoadSerpAPIConfig() SerpAPIConfig {
	return LoadSerpAPIConfigFromEnv(os.Getenv)
}

func LoadSerpAPIConfigFromEnv(getEnv func(string) string) SerpAPIConfig {
	apiKey := strings.TrimSpace(getEnv("SERPAPI_API_KEY"))
	engine := strings.TrimSpace(getEnv("SERPAPI_ENGINE"))
	if engine == "" {
		engine = DefaultSerpAPIEngine
	}

	return SerpAPIConfig{
		APIKey: apiKey,
		Engine: engine,
	}
}

func (c SerpAPIConfig) Validate() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("SERPAPI_API_KEY is required")
	}
	if strings.TrimSpace(c.Engine) == "" {
		return fmt.Errorf("SERPAPI_ENGINE is required")
	}
	return nil
}
