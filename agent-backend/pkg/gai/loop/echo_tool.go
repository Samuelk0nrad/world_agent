package loop

import "strings"

type EchoTool struct{}

func NewEchoTool() *EchoTool {
	return &EchoTool{}
}

func (t *EchoTool) Name() string {
	return "echo"
}

func (t *EchoTool) Description() string {
	return "Returns the same text passed in arguments."
}

func (t *EchoTool) Params() string {
	return `{"arguments":{"type":"string","description":"Text to echo back"}}`
}

func (t *EchoTool) Function(req *ToolRequest) (*ToolResponse, error) {
	text := strings.TrimSpace(req.ArgsString())
	if text == "" {
		text = "(empty)"
	}
	return &ToolResponse{Text: text}, nil
}
