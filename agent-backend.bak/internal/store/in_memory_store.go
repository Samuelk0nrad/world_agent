package store

import (
	"fmt"
	"sync"
	"time"

	"worldagent/agent-backend/internal/memory"
)

type InMemoryStore struct {
	mu      sync.Mutex
	entries []memory.Entry
	nextSeq int64
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{nextSeq: 1}
}

func (s *InMemoryStore) Append(source, content string) (memory.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := memory.Entry{
		ID:        fmt.Sprintf("mem-%d", s.nextSeq),
		Source:    source,
		Content:   content,
		CreatedAt: time.Now().UTC(),
		Sequence:  s.nextSeq,
	}
	s.nextSeq++
	s.entries = append(s.entries, entry)

	return entry, nil
}

func (s *InMemoryStore) ListAll() ([]memory.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]memory.Entry, len(s.entries))
	copy(out, s.entries)
	return out, nil
}

func (s *InMemoryStore) ListSince(sequence int64) ([]memory.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]memory.Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.Sequence > sequence {
			out = append(out, entry)
		}
	}

	return out, nil
}

func (s *InMemoryStore) LatestSequence() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.nextSeq <= 1 {
		return 0, nil
	}
	return s.nextSeq - 1, nil
}

var _ MemoryStore = (*InMemoryStore)(nil)
