package store

import "worldagent/agent-backend/internal/memory"

type MemoryStore interface {
	Append(source, content string) (memory.Entry, error)
	ListAll() ([]memory.Entry, error)
	ListSince(sequence int64) ([]memory.Entry, error)
	LatestSequence() (int64, error)
}
