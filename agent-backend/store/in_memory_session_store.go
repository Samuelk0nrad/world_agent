package store

import (
	"fmt"
	"sync"

	"agent-backend/gai/context"
)

type Session struct {
	messages []context.Message
}

type InMemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[int]*Session
}

func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[int]*Session),
		mu:       sync.RWMutex{},
	}
}

func (s *InMemorySessionStore) GetSession(sessionID int) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("%v with id: %d", context.ErrSessionNotFound, sessionID)
	}
	return nil
}

func (s *InMemorySessionStore) GetMessages(sessionID int, limit int, offset int) ([]context.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("%v with id: %d", context.ErrSessionNotFound, sessionID)
	}

	messages := session.messages
	if offset >= len(messages) {
		return []context.Message{}, nil
	}

	end := offset + limit
	end = min(end, len(messages))

	return messages[offset:end], nil
}
