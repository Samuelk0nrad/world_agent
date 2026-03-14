package config

import "testing"

func TestLoadGeminiConfigFromEnvUsesDefaults(t *testing.T) {
	t.Parallel()

	cfg := LoadGeminiConfigFromEnv(func(string) string { return "" })
	if cfg.APIKey != "" {
		t.Fatalf("expected empty API key by default, got %q", cfg.APIKey)
	}
	if cfg.Model != DefaultGeminiModel {
		t.Fatalf("expected default model %q, got %q", DefaultGeminiModel, cfg.Model)
	}
}

func TestLoadGeminiConfigFromEnvReadsValues(t *testing.T) {
	t.Parallel()

	cfg := LoadGeminiConfigFromEnv(func(key string) string {
		switch key {
		case "GEMINI_API_KEY":
			return "  api-key  "
		case "GEMINI_MODEL":
			return " gemini-2.0-flash "
		default:
			return ""
		}
	})

	if cfg.APIKey != "api-key" {
		t.Fatalf("expected trimmed API key, got %q", cfg.APIKey)
	}
	if cfg.Model != "gemini-2.0-flash" {
		t.Fatalf("expected trimmed model value, got %q", cfg.Model)
	}
}

func TestGeminiConfigValidateRequiresAPIKey(t *testing.T) {
	t.Parallel()

	err := (GeminiConfig{Model: DefaultGeminiModel}).Validate()
	if err == nil {
		t.Fatal("expected missing API key validation error")
	}
}
