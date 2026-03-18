package loop

type ToolParams struct {
	Name string
	Args map[string]any
}

type ToolResponse struct {
	Text string
}

type Tool interface {
	Name() string
	Description() string
	Params() string
	Function(params *ToolParams) (*ToolResponse, error)
}
