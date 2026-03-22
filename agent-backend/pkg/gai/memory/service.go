package memory

import (
	"strings"
)

type MemoryService struct {
	repo Repository
}

func (m *MemoryService) AddMessage(content string, role Role, sessionID int) (Message, error) {
	return m.repo.AddMessages(content, role, sessionID)
}

func (m *MemoryService) GetMessages(sessionID string) ([]Message, error) {
	return m.repo.GetMessagesBySession(sessionID)
}

func (m *MemoryService) EnrichPrompt(prompt string, sessionID string) (string, error) {
	messages, err := m.repo.GetMessagesBySession(sessionID)
	if err != nil {
		return prompt, err
	}

	var builder strings.Builder
	builder.WriteString(prompt)

	builder.WriteString("<conversation>")
	RenderMessages(messages, &builder)
	builder.WriteString("</conversation>")

	return builder.String(), nil
}
