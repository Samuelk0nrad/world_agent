package loop

import (
	"context"
	"strconv"
	"strings"

	"agent-backend/pkg/gai/ai"
)

type Agent struct {
	Model        ai.Model
	Tools        []Tool
	Messages     []Message
	SystemPrompt string
}

func NewAgent(model ai.Model, tools []Tool, systemPrompt string) *Agent {
	agent := &Agent{
		Model:        model,
		Tools:        tools,
		SystemPrompt: systemPrompt,
	}

	agent.addMessage(Message{systemPrompt, RoleSystem})

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
	request := ai.AIRequest{SystemPrompt: buildSystemPrompt(a.Messages), Prompt: "\n" + string(message.Role) + ": \n\t" + message.Text}
	res, err := a.Model.Generate(ctx, request)
	if err != nil {
		return err
	}

	a.addMessage(message)

	assistantMessage := Message{Text: res.Text, Role: RoleAssistant}
	a.addMessage(assistantMessage)

	response.WriteString("\n\n")
	response.WriteString("Agent: \n")
	response.WriteString("\t")
	response.WriteString(assistantMessage.Text)

	toolReq, tCall := detectToolCall(res.Text)

	if tCall {
		res, err := callTool(toolReq, a.Tools)

		response.WriteString("\n\n")
		response.WriteString("Tool ")
		response.WriteString(toolReq.ID)
		response.WriteString(" ")
		response.WriteString(toolReq.Args)
		response.WriteString(":\n")
		response.WriteString("\t")
		response.WriteString(res.Text)

		toolMessage := Message{res.Text, RoleTool}
		if err != nil {
			toolMessage = Message{err.Error(), RoleTool}
		}
		return a.Loop(ctx, toolMessage, response)
	} else {
		return nil
	}
}

func (a *Agent) addMessage(newMessage Message) {
	a.Messages = append(a.Messages, newMessage)
}

func buildSystemPrompt(messages []Message) string {
	var builder strings.Builder

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
