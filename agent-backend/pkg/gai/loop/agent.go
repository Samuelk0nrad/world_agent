package loop

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"agent-backend/pkg/gai/ai"
)

const defaultMaxLoopIterations = 8

type Agent struct {
	Model             ai.Model
	Tools             []Tool
	Messages          []Message
	SystemPrompt      string
	MaxLoopIterations int
}

func NewAgent(model ai.Model, tools []Tool, systemPrompt string) *Agent {
	agent := &Agent{
		Model:             model,
		Tools:             tools,
		SystemPrompt:      systemPrompt,
		MaxLoopIterations: defaultMaxLoopIterations,
	}

	return agent
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

	userMessage := Message{Text: prompt, Role: RoleUser}
	var response strings.Builder
	err := a.Loop(ctx, userMessage, &response)
	if err != nil {
		return "", err
	}
	return response.String(), nil
}

func (a *Agent) Loop(ctx context.Context, message Message, response *strings.Builder) error {
	if response == nil {
		return ErrNilResponseBuilder
	}

	if a.MaxLoopIterations <= 0 {
		a.MaxLoopIterations = defaultMaxLoopIterations
	}

	a.addMessage(message)

	for i := 0; i < a.MaxLoopIterations; i++ {
		request := ai.AIRequest{SystemPrompt: buildSystemPrompt(a.SystemPrompt, a.Messages)}
		res, err := a.Model.Generate(ctx, request)
		if err != nil {
			return err
		}

		assistantMessage := Message{Text: res.Text, Role: RoleAssistant}
		a.addMessage(assistantMessage)

		response.WriteString("\n\n")
		response.WriteString("Agent: \n")
		response.WriteString("\t")
		response.WriteString(assistantMessage.Text)

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

		toolMessage := Message{Text: toolResultText, Role: RoleTool}
		a.addMessage(toolMessage)
	}

	return fmt.Errorf("%w: limit=%d", ErrMaxIterations, a.MaxLoopIterations)
}

func (a *Agent) addMessage(newMessage Message) {
	a.Messages = append(a.Messages, newMessage)
}

func buildSystemPrompt(systemPrompt string, messages []Message) string {
	var builder strings.Builder

	if strings.TrimSpace(systemPrompt) != "" {
		builder.WriteString(systemPrompt)
		builder.WriteString("\n\n")
	}

	builder.WriteString("<conversation>")
	builder.WriteString(renderMessages(messages))
	builder.WriteString("</conversation>")

	return builder.String()
}

func renderMessages(messages []Message) string {
	var builder strings.Builder

	for i, m := range messages {
		builder.WriteString("<")
		builder.WriteString(string(m.Role))
		builder.WriteString(" key=")
		builder.WriteString(strconv.Itoa(i))
		builder.WriteString(">\n")
		builder.WriteString(m.Text)
		builder.WriteString("\n")
		builder.WriteString("</")
		builder.WriteString(string(m.Role))
		builder.WriteString(">")
	}

	return builder.String()
}
