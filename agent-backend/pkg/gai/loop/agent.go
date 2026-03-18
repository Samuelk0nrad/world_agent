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
	return &Agent{
		Model:        model,
		Tools:        tools,
		SystemPrompt: systemPrompt,
	}
}

func (a *Agent) FollowUp(ctx context.Context, prompt string) (*Message, error) {
	userMessage := Message{Text: prompt, Role: "user"}
	a.addMessage(userMessage)

	historyPrompt := a.SystemPrompt + "\nThis is the conversation history:\n" + a.getAllMessages()
	request := ai.AIRequest{SystemPrompt: historyPrompt}

	res, err := a.Model.Generate(ctx, request)
	if err != nil {
		return nil, err
	}

	agentMessage := Message{Text: res.Text, Role: "agent"}
	a.addMessage(agentMessage)

	return &agentMessage, nil
}

func (a *Agent) addMessage(newMessage Message) {
	a.Messages = append(a.Messages, newMessage)
}

func (a *Agent) getAllMessages() string {
	var builder strings.Builder

	for i, m := range a.Messages {
		builder.WriteString(strconv.Itoa(i))
		builder.WriteString(" (")
		builder.WriteString(m.Role)
		builder.WriteString("):\n")
		builder.WriteString("\t")
		builder.WriteString(m.Text)
		builder.WriteString("\n")
	}

	return builder.String()
}
