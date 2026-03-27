package schema

type AgentRequest struct {
	Prompt    string `json:"prompt"`
	SessionID int    `json:"sessionId"`
}
