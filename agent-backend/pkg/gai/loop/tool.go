package loop

import "encoding/json"

type ToolArg struct {
	ArgType     string `json:"type"`
	Description string `json:"description"`
}

type ToolArgs struct {
	Args map[string]ToolArg `json:"arguments"`
}

type ToolResponse struct {
	Text string
}

type ToolRequest struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Args string `json:"arguments"`
}

type Tool interface {
	Name() string
	Description() string
	Params() string
	Function(req *ToolRequest) (*ToolResponse, error)
}

func detectToolCall(s string) (*ToolRequest, bool) {
	var tr ToolRequest
	if err := json.Unmarshal([]byte(s), &tr); err != nil {
		return nil, false
	}
	if tr.Type == "function" && tr.ID != "" {
		return &tr, true
	}
	return nil, false
}

func callTool(req *ToolRequest, tools []Tool) (*ToolResponse, error) {
	var callTool Tool

	for _, tool := range tools {
		if tool.Name() == req.ID {
			callTool = tool
		}
	}

	return callTool.Function(req)
}
