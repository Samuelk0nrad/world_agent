package loop

import (
	"encoding/json"
	"fmt"
)

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
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Args json.RawMessage `json:"arguments"`
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

func (r *ToolRequest) ArgsString() string {
	if r == nil {
		return ""
	}
	if len(r.Args) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(r.Args, &s); err == nil {
		return s
	}
	return string(r.Args)
}

func callTool(req *ToolRequest, tools []Tool) (*ToolResponse, error) {
	if req == nil || req.ID == "" || req.Type != "function" {
		return nil, ErrInvalidToolRequest
	}

	for _, tool := range tools {
		if tool.Name() == req.ID {
			return tool.Function(req)
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrToolNotFound, req.ID)
}
