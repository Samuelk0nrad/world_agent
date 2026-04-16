package store

import (
	"fmt"
	"sync"
	"time"

	aicontext "agent-backend/gai/context"
)

type Session struct {
	messages []aicontext.Message
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
		return fmt.Errorf("%w with id: %d", aicontext.ErrSessionNotFound, sessionID)
	}
	return nil
}

func (s *InMemorySessionStore) GetMessages(sessionID int, limit int, offset int) ([]aicontext.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("%w with id: %d", aicontext.ErrSessionNotFound, sessionID)
	}

	messages := session.messages
	if offset >= len(messages) {
		return []aicontext.Message{}, nil
	}

	// 0, 1, 2, 3, 4, 5, 6, 7, 8
	// len: 9
	// eg:
	// 	offset: 0
	// 	limit: 5
	// 	start: 9 - 0 = 9
	// 	end: 9 - 5 = 4
	end := len(messages) - offset
	start := end - limit
	start = max(start, 0)

	return messages[start:end], nil
}

func (s *InMemorySessionStore) CreateSession() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newID := len(s.sessions) + 1
	s.sessions[newID] = &Session{
		messages: []aicontext.Message{},
	}
	return newID, nil
}

func (s *InMemorySessionStore) AddMessage(sessionID int, message aicontext.Message) (aicontext.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addMessage(sessionID, message)
}

func (s *InMemorySessionStore) AddMessages(sessionID int, messages []aicontext.Message) ([]aicontext.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var storeMessages []aicontext.Message

	_, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("%w with id: %d", aicontext.ErrSessionNotFound, sessionID)
	}

	for _, m := range messages {
		msg, err := s.addMessage(sessionID, m)
		if err != nil {
			return nil, err
		}
		storeMessages = append(storeMessages, msg)
	}
	return storeMessages, nil
}

func (s *InMemorySessionStore) addMessage(sessionID int, message aicontext.Message) (aicontext.Message, error) {
	session, exists := s.sessions[sessionID]
	if !exists {
		return message, fmt.Errorf("%w with id: %d", aicontext.ErrSessionNotFound, sessionID)
	}
	message.ID = len(session.messages) + 1
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now()
	}
	message.SessionID = sessionID
	session.messages = append(session.messages, message)
	return message, nil
}
