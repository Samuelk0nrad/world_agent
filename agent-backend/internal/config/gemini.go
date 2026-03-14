package config

import (
	"fmt"
	"os"
	"strings"
)

const DefaultGeminiModel = "gemini-1.5-flash"

type GeminiConfig struct {
	APIKey string
	Model  string
}

func LoadGeminiConfig() GeminiConfig {
	return LoadGeminiConfigFromEnv(os.Getenv)
}

func LoadGeminiConfigFromEnv(getEnv func(string) string) GeminiConfig {
	apiKey := strings.TrimSpace(getEnv("GEMINI_API_KEY"))
	model := strings.TrimSpace(getEnv("GEMINI_MODEL"))
	if model == "" {
		model = DefaultGeminiModel
	}

	return GeminiConfig{
		APIKey: apiKey,
		Model:  model,
	}
}

func (c GeminiConfig) Validate() error {
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("GEMINI_API_KEY is required")
	}
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("GEMINI_MODEL is required")
	}
	return nil
}
