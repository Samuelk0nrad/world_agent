package server

import (
	"os"

	"worldagent/agent-backend/internal/connectors"
	"worldagent/agent-backend/internal/llm"
	"worldagent/agent-backend/internal/policy"
)

type Config struct {
	Port              string
	MemoryFile        string
	LLMConnector      string
	Policy            policy.Gate
	GeminiResponder   llm.Responder
	ConnectorRegistry *connectors.Registry
}

func LoadConfig() Config {
	port := os.Getenv("AGENT_BACKEND_PORT")
	if port == "" {
		port = "8088"
	}

	memoryFile := os.Getenv("AGENT_MEMORY_FILE")
	if memoryFile == "" {
		memoryFile = "./data/memory.jsonl"
	}

	llmConnector := os.Getenv("AGENT_LLM_CONNECTOR")

	return Config{
		Port:         port,
		MemoryFile:   memoryFile,
		LLMConnector: llmConnector,
		Policy:       policy.LoadFromEnv(),
	}
}
