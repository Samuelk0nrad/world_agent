package config

import "testing"

func TestLoadGoogleOAuthConfigFromEnvReadsValues(t *testing.T) {
	t.Parallel()

	cfg := LoadGoogleOAuthConfigFromEnv(func(key string) string {
		switch key {
		case "GOOGLE_CLIENT_ID":
			return " client-id "
		case "GOOGLE_CLIENT_SECRET":
			return " client-secret "
		case "GOOGLE_REDIRECT_URL":
			return " https://example.com/oauth/callback "
		default:
			return ""
		}
	})

	if cfg.ClientID != "client-id" {
		t.Fatalf("expected trimmed client id, got %q", cfg.ClientID)
	}
	if cfg.ClientSecret != "client-secret" {
		t.Fatalf("expected trimmed client secret, got %q", cfg.ClientSecret)
	}
	if cfg.RedirectURL != "https://example.com/oauth/callback" {
		t.Fatalf("expected trimmed redirect URL, got %q", cfg.RedirectURL)
	}
}

func TestGoogleOAuthConfigValidateRequiresFields(t *testing.T) {
	t.Parallel()

	err := (GoogleOAuthConfig{}).Validate()
	if err == nil || err.Error() != "GOOGLE_CLIENT_ID is required" {
		t.Fatalf("expected missing client id error, got %v", err)
	}

	err = (GoogleOAuthConfig{ClientID: "id"}).Validate()
	if err == nil || err.Error() != "GOOGLE_CLIENT_SECRET is required" {
		t.Fatalf("expected missing client secret error, got %v", err)
	}

	err = (GoogleOAuthConfig{ClientID: "id", ClientSecret: "secret"}).Validate()
	if err == nil || err.Error() != "GOOGLE_REDIRECT_URL is required" {
		t.Fatalf("expected missing redirect URL error, got %v", err)
	}
}
