package schema

import "agent-backend/pkg/gai/memory"

type AgentResponse struct {
	Prompt   string           `json:"prompt"`
	Message  string           `json:"message"`
	Messages []memory.Message `json:"messages"`
}
