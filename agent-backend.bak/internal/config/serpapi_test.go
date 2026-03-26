package config

import "testing"

func TestLoadSerpAPIConfigFromEnvUsesDefaults(t *testing.T) {
	t.Parallel()

	cfg := LoadSerpAPIConfigFromEnv(func(string) string { return "" })
	if cfg.APIKey != "" {
		t.Fatalf("expected empty API key by default, got %q", cfg.APIKey)
	}
	if cfg.Engine != DefaultSerpAPIEngine {
		t.Fatalf("expected default engine %q, got %q", DefaultSerpAPIEngine, cfg.Engine)
	}
}

func TestLoadSerpAPIConfigFromEnvReadsValues(t *testing.T) {
	t.Parallel()

	cfg := LoadSerpAPIConfigFromEnv(func(key string) string {
		switch key {
		case "SERPAPI_API_KEY":
			return "  api-key  "
		case "SERPAPI_ENGINE":
			return " bing "
		default:
			return ""
		}
	})

	if cfg.APIKey != "api-key" {
		t.Fatalf("expected trimmed API key, got %q", cfg.APIKey)
	}
	if cfg.Engine != "bing" {
		t.Fatalf("expected trimmed engine value, got %q", cfg.Engine)
	}
}

func TestSerpAPIConfigValidateRequiresAPIKey(t *testing.T) {
	t.Parallel()

	err := (SerpAPIConfig{Engine: DefaultSerpAPIEngine}).Validate()
	if err == nil {
		t.Fatal("expected missing API key validation error")
	}
}
