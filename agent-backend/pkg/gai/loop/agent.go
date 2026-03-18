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

func (a *Agent) FollowUp(ctx context.Context, prompt string) (*Message, error) {
	if a == nil {
		return nil, ErrNilAgent
	}
	if a.Model == nil {
		return nil, ErrModelNotConfigured
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, ErrEmptyPrompt
	}

	userMessage := Message{Text: prompt, Role: RoleUser}
	a.addMessage(userMessage)

	request := ai.AIRequest{SystemPrompt: buildSystemPrompt(a.Messages)}
	res, err := a.Model.Generate(ctx, request)
	if err != nil {
		return nil, err
	}

	assistantMessage := Message{Text: res.Text, Role: RoleAssistant}
	a.addMessage(assistantMessage)

	return &assistantMessage, nil
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
