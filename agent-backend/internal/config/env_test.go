package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewViperFromEnvReadsConfigFileAndEnvOverride(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	configFile := filepath.Join(tempDir, ".env")
	content := []byte("AGENT_BACKEND_PORT=7777\nGEMINI_MODEL=file-model\n")
	if err := os.WriteFile(configFile, content, 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("AGENT_BACKEND_PORT", "8888")

	cfg, err := NewViperFromEnv()
	if err != nil {
		t.Fatalf("NewViperFromEnv returned error: %v", err)
	}

	if got := cfg.GetString("AGENT_BACKEND_PORT"); got != "8888" {
		t.Fatalf("expected env override for AGENT_BACKEND_PORT=8888, got %q", got)
	}
	if got := cfg.GetString("GEMINI_MODEL"); got != "file-model" {
		t.Fatalf("expected GEMINI_MODEL from file, got %q", got)
	}
}

func TestNewViperFromEnvReturnsErrorForMissingConfigFile(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	_, err = NewViperFromEnv()
	if err == nil {
		t.Fatal("expected error when .env is missing")
	}
}
