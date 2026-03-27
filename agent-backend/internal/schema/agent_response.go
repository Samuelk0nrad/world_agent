package schema

type AgentResponse struct {
	Model   string `json:"model"`
	Content string `json:"content"`
	Tokens  int    `json:"tokens"`
}
