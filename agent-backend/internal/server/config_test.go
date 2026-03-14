package server

import (
	"testing"

	"github.com/spf13/viper"
)

func TestLoadConfigReadsLLMConnector(t *testing.T) {
	cfgSource := viper.New()
	cfgSource.Set("AGENT_BACKEND_PORT", "9010")
	cfgSource.Set("AGENT_MEMORY_FILE", "/tmp/test-memory.jsonl")
	cfgSource.Set("AGENT_LLM_CONNECTOR", "gemini")
	cfgSource.Set("AGENT_LOG_LEVEL", "debug")
	cfgSource.Set("AGENT_LOG_FORMAT", "json")
	cfgSource.Set("AGENT_LOG_EVENTS_ENABLED", true)
	cfgSource.Set("AGENT_LOG_API_ENABLED", true)
	cfgSource.Set("AGENT_LOG_INCLUDE_PAYLOAD", true)
	cfgSource.Set("AGENT_LOG_EVENT_BUFFER", 5000)

	cfg := LoadConfigFromViper(cfgSource)
	if cfg.Port != "9010" {
		t.Fatalf("expected port 9010, got %q", cfg.Port)
	}
	if cfg.MemoryFile != "/tmp/test-memory.jsonl" {
		t.Fatalf("expected memory file path from env, got %q", cfg.MemoryFile)
	}
	if cfg.LLMConnector != "gemini" {
		t.Fatalf("expected LLM connector gemini, got %q", cfg.LLMConnector)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("expected log level debug, got %q", cfg.LogLevel)
	}
	if !cfg.LogEventsEnabled || !cfg.LogAPIEnabled || !cfg.LogIncludePayload {
		t.Fatalf("expected logging toggles enabled, got events=%v api=%v payload=%v", cfg.LogEventsEnabled, cfg.LogAPIEnabled, cfg.LogIncludePayload)
	}
	if cfg.LogEventBuffer != 5000 {
		t.Fatalf("expected log event buffer 5000, got %d", cfg.LogEventBuffer)
	}
}

func TestLoadConfigDefaultsWithoutLLMConnector(t *testing.T) {
	cfgSource := viper.New()

	cfg := LoadConfigFromViper(cfgSource)
	if cfg.Port != "8088" {
		t.Fatalf("expected default port 8088, got %q", cfg.Port)
	}
	if cfg.MemoryFile != "./data/memory.jsonl" {
		t.Fatalf("expected default memory file, got %q", cfg.MemoryFile)
	}
	if cfg.LLMConnector != "" {
		t.Fatalf("expected empty LLM connector by default, got %q", cfg.LLMConnector)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("expected default log level info, got %q", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Fatalf("expected default log format json, got %q", cfg.LogFormat)
	}
	if !cfg.LogEventsEnabled || !cfg.LogAPIEnabled {
		t.Fatalf("expected logging enabled defaults, got events=%v api=%v", cfg.LogEventsEnabled, cfg.LogAPIEnabled)
	}
	if cfg.LogIncludePayload {
		t.Fatalf("expected payload logging disabled by default")
	}
	if cfg.LogEventBuffer != 2000 {
		t.Fatalf("expected default log event buffer 2000, got %d", cfg.LogEventBuffer)
	}
}
