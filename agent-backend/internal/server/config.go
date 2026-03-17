package server

import (
	"strings"

	"github.com/spf13/viper"
	"worldagent/agent-backend/internal/config"
	"worldagent/agent-backend/internal/connectors"
	"worldagent/agent-backend/internal/llm"
	"worldagent/agent-backend/internal/policy"
)

type Config struct {
	Port              string
	MemoryFile        string
	LLMConnector      string
	LogLevel          string
	LogFormat         string
	LogEventsEnabled  bool
	LogAPIEnabled     bool
	LogIncludePayload bool
	LogEventBuffer    int
	Policy            policy.Gate
	GeminiResponder   llm.Responder
	ConnectorRegistry *connectors.Registry
}

func LoadConfig() Config {
	env := config.MustViper()
	return LoadConfigFromViper(env)
}

func LoadConfigFromViper(env *viper.Viper) Config {
	if env == nil {
		panic("LoadConfigFromViper requires non-nil viper config")
	}
	port := strings.TrimSpace(env.GetString("AGENT_BACKEND_PORT"))
	if port == "" {
		port = "8088"
	}

	memoryFile := strings.TrimSpace(env.GetString("AGENT_MEMORY_FILE"))
	if memoryFile == "" {
		memoryFile = "./data/memory.jsonl"
	}

	llmConnector := strings.TrimSpace(env.GetString("AGENT_LLM_CONNECTOR"))
	logLevel := strings.TrimSpace(env.GetString("AGENT_LOG_LEVEL"))
	if logLevel == "" {
		logLevel = "info"
	}
	logFormat := strings.TrimSpace(env.GetString("AGENT_LOG_FORMAT"))
	if logFormat == "" {
		logFormat = "json"
	}
	logEventsEnabled := env.GetBool("AGENT_LOG_EVENTS_ENABLED")
	if !env.IsSet("AGENT_LOG_EVENTS_ENABLED") {
		logEventsEnabled = true
	}
	logAPIEnabled := env.GetBool("AGENT_LOG_API_ENABLED")
	if !env.IsSet("AGENT_LOG_API_ENABLED") {
		logAPIEnabled = true
	}
	logIncludePayload := env.GetBool("AGENT_LOG_INCLUDE_PAYLOAD")
	if !env.IsSet("AGENT_LOG_INCLUDE_PAYLOAD") {
		logIncludePayload = true
	}
	logEventBuffer := env.GetInt("AGENT_LOG_EVENT_BUFFER")
	if logEventBuffer <= 0 {
		logEventBuffer = 2000
	}

	return Config{
		Port:              port,
		MemoryFile:        memoryFile,
		LLMConnector:      llmConnector,
		LogLevel:          logLevel,
		LogFormat:         logFormat,
		LogEventsEnabled:  logEventsEnabled,
		LogAPIEnabled:     logAPIEnabled,
		LogIncludePayload: logIncludePayload,
		LogEventBuffer:    logEventBuffer,
		Policy:            policy.LoadFromViper(env),
	}
}
