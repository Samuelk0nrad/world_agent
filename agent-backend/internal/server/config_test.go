package server

import "testing"

func TestLoadConfigReadsLLMConnector(t *testing.T) {
	t.Setenv("AGENT_BACKEND_PORT", "9010")
	t.Setenv("AGENT_MEMORY_FILE", "/tmp/test-memory.jsonl")
	t.Setenv("AGENT_LLM_CONNECTOR", "gemini")

	cfg := LoadConfig()
	if cfg.Port != "9010" {
		t.Fatalf("expected port 9010, got %q", cfg.Port)
	}
	if cfg.MemoryFile != "/tmp/test-memory.jsonl" {
		t.Fatalf("expected memory file path from env, got %q", cfg.MemoryFile)
	}
	if cfg.LLMConnector != "gemini" {
		t.Fatalf("expected LLM connector gemini, got %q", cfg.LLMConnector)
	}
}

func TestLoadConfigDefaultsWithoutLLMConnector(t *testing.T) {
	t.Setenv("AGENT_BACKEND_PORT", "")
	t.Setenv("AGENT_MEMORY_FILE", "")
	t.Setenv("AGENT_LLM_CONNECTOR", "")

	cfg := LoadConfig()
	if cfg.Port != "8088" {
		t.Fatalf("expected default port 8088, got %q", cfg.Port)
	}
	if cfg.MemoryFile != "./data/memory.jsonl" {
		t.Fatalf("expected default memory file, got %q", cfg.MemoryFile)
	}
	if cfg.LLMConnector != "" {
		t.Fatalf("expected empty LLM connector by default, got %q", cfg.LLMConnector)
	}
}
