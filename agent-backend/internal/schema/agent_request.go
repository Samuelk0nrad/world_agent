package schema

type AgentRequest struct {
	Prompt    string `json:"prompt" binding:"required"`
	SessionID int    `json:"sessionId" binding:"required,gt=0"`
}
