package loop

import (
	"context"
	"fmt"
	"strings"

	"agent-backend/pkg/gai/ai"
	"agent-backend/pkg/gai/memory"
)

const (
	defaultMaxLoopIterations = 8
	defaultToolSystemPrompt  = `When a tool is required, respond ONLY with one JSON object using this exact shape:
{"id":"<tool-name>","type":"function","arguments":{...}}
Rules:
- No prose, markdown, or code fences.
- "id" must match one tool name from <tools>.
- "type" must be "function".
- "arguments" must be a JSON object that satisfies the tool signature.`
	defaultMaxMessages = 100
)

type Agent struct {
	Model             ai.Model
	Tools             []Tool
	Messages          []memory.Message
	BaseSystemPrompt  string
	ToolSystemPrompt  string
	MaxLoopIterations int
	MaxMessages       int
	MemorySystem      memory.Memory
}

func NewAgent(model ai.Model, tools []Tool, systemPrompt string, sessionID int) (*Agent, error) {
	m, err := memory.NewMemory(sessionID)
	if err != nil {
		return nil, err
	}
	agent := &Agent{
		Model:             model,
		Tools:             tools,
		BaseSystemPrompt:  systemPrompt,
		ToolSystemPrompt:  defaultToolSystemPrompt,
		MaxLoopIterations: defaultMaxLoopIterations,
		MaxMessages:       defaultMaxMessages,
		MemorySystem:      m,
	}

	return agent, nil
}

func NewAgentWithPrompts(model ai.Model, tools []Tool, baseSystemPrompt, toolSystemPrompt string, sessionID int) (*Agent, error) {
	agent, err := NewAgent(model, tools, baseSystemPrompt, sessionID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(toolSystemPrompt) != "" {
		agent.ToolSystemPrompt = toolSystemPrompt
	}
	return agent, nil
}

func NewAgentFromPromptFiles(model ai.Model, tools []Tool, basePromptPath, toolPromptPath string, sessionID int) (*Agent, error) {
	basePrompt, err := LoadPromptFromFile(basePromptPath)
	if err != nil {
		return nil, err
	}
	toolPrompt, err := LoadOptionalPromptFromFile(toolPromptPath, defaultToolSystemPrompt)
	if err != nil {
		return nil, err
	}
	return NewAgentWithPrompts(model, tools, basePrompt, toolPrompt, sessionID)
}

func (a *Agent) FollowUp(ctx context.Context, prompt string) (string, error) {
	if a == nil {
		return "", ErrNilAgent
	}
	if a.Model == nil {
		return "", ErrModelNotConfigured
	}
	if strings.TrimSpace(prompt) == "" {
		return "", ErrEmptyPrompt
	}

	userMessage := memory.Message{Content: prompt, Role: memory.RoleUser}
	var response strings.Builder
	err := a.Loop(ctx, userMessage, &response)
	if err != nil {
		return "", err
	}
	return response.String(), nil
}

func (a *Agent) Loop(ctx context.Context, message memory.Message, response *strings.Builder) error {
	if response == nil {
		return ErrNilResponseBuilder
	}

	if a.MaxLoopIterations <= 0 {
		a.MaxLoopIterations = defaultMaxLoopIterations
	}

	_, err := a.MemorySystem.AddMessage(message.Content, message.Role)
	if err != nil {
		return err
	}

	for i := 0; i < a.MaxLoopIterations; i++ {
		request := ai.AIRequest{
			SystemPrompt: buildSystemPrompt(a.BaseSystemPrompt, a.ToolSystemPrompt, a.Tools),
		}
		res, err := a.Model.Generate(ctx, request)
		if err != nil {
			return err
		}

		message, err := a.MemorySystem.AddMessage(res.Text, memory.RoleAssistant)
		if err != nil {
			return err
		}

		response.WriteString("\n\n")
		response.WriteString("Agent: \n")
		response.WriteString("\t")
		response.WriteString(message.Content)

		toolReq, tCall := detectToolCall(res.Text)
		if !tCall {
			return nil
		}

		toolRes, err := callTool(toolReq, a.Tools)
		toolResultText := ""
		if err != nil {
			toolResultText = err.Error()
		} else if toolRes != nil {
			toolResultText = toolRes.Text
		}

		response.WriteString("\n\n")
		response.WriteString("Tool ")
		response.WriteString(toolReq.ID)
		response.WriteString(" ")
		response.WriteString(toolReq.ArgsString())
		response.WriteString(":\n")
		response.WriteString("\t")
		response.WriteString(toolResultText)

		a.MemorySystem.AddMessage(toolResultText, memory.RoleTool)

		if err != nil {
			return nil
		}
	}

	return fmt.Errorf("%w: limit=%d", ErrMaxIterations, a.MaxLoopIterations)
}

func buildSystemPrompt(baseSystemPrompt, toolSystemPrompt string, tools []Tool) string {
	var builder strings.Builder

	if strings.TrimSpace(baseSystemPrompt) != "" {
		builder.WriteString(baseSystemPrompt)
		builder.WriteString("\n\n")
	}

	if len(tools) > 0 {
		prompt := strings.TrimSpace(toolSystemPrompt)
		if prompt == "" {
			prompt = defaultToolSystemPrompt
		}
		builder.WriteString(prompt)
		builder.WriteString("<tools>\n")
		builder.WriteString(RenderToolSignatures(tools))
		builder.WriteString("\n</tools>")
	}

	return builder.String()
}
