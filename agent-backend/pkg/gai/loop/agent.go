package loop

import (
	"context"
	"fmt"
	"strconv"

	"agent-backend/pkg/gai/ai"
)

type Agent struct {
	Model       ai.Model
	Tools       []Tool
	Messages    []Message
	SystemPromt string
}

func NewAgent(model ai.Model, tools []Tool, systemPrompt string) *Agent {
	return &Agent{
		Model:       model,
		Tools:       tools,
		SystemPromt: systemPrompt,
	}
}

func (a *Agent) FollowUp(ctx context.Context, promt string) (*Message, error) {
	var userMessage Message
	userMessage.Text = promt
	userMessage.Role = "user"

	a.addMessage(userMessage)

	systemPromt := a.SystemPromt
	systemPromt += "\nThis is the conversation history: \n"
	systemPromt += a.getAllMessages()

	var request ai.AIRequest
	request.Promt = systemPromt + "\n"
	request.Promt += promt

	var message Message
	res, err := a.Model.Generate(ctx, request)
	if err != nil {
		return nil, err
	}

	message.Text = res.Text
	message.Role = "agent"

	a.addMessage(message)

	return &message, err
}

func (a *Agent) addMessage(newMessage Message) {
	fmt.Println("New Message from: ", newMessage.Role, " with content: ", newMessage.Text)

	a.Messages = append(a.Messages, newMessage)
}

func (a *Agent) getAllMessages() string {
	var s string
	for i, m := range a.Messages {
		cm := strconv.Itoa(i) + " (" + m.Role + "):\n"
		cm += "	" + m.Text + "\n"
		s += cm
	}
	return s
}
