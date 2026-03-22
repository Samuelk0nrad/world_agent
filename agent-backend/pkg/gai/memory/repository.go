package memory

import (
	"strconv"
	"time"
)

type Repository struct{}

var (
	counter  int = 0
	messages []Message
)

func (r *Repository) GetMessagesBySession(sessionID string) ([]Message, error) {
	id, err := strconv.Atoi(sessionID)
	if err != nil {
		return nil, ErrSessionIDInValide
	}

	var res []Message
	for _, message := range messages {
		if message.SessionID == id {
			res = append(res, message)
		}
	}

	return res, nil
}

func (r *Repository) AddMessages(content string, role Role, sessionID int) (Message, error) {
	message := Message{
		ID:        counter,
		SessionID: sessionID,
		CreatedAt: time.Now(),
		Content:   content,
		Role:      role,
	}
	counter++

	messages = append(messages, message)
	return message, nil
}
