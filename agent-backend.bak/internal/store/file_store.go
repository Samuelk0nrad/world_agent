package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"worldagent/agent-backend/internal/memory"
)

type FileStore struct {
	path string

	mu          sync.Mutex
	initialized bool
	nextSeq     int64
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (s *FileStore) ensurePath() error {
	dir := filepath.Dir(s.path)
	return os.MkdirAll(dir, 0o755)
}

func (s *FileStore) Append(source, content string) (memory.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensurePath(); err != nil {
		return memory.Entry{}, err
	}
	if err := s.initLocked(); err != nil {
		return memory.Entry{}, err
	}

	seq := s.nextSeq
	s.nextSeq++

	entry := memory.Entry{
		ID:        fmt.Sprintf("mem-%d", seq),
		Source:    source,
		Content:   content,
		CreatedAt: time.Now().UTC(),
		Sequence:  seq,
	}

	raw, err := json.Marshal(entry)
	if err != nil {
		return memory.Entry{}, err
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return memory.Entry{}, err
	}
	defer f.Close()

	if _, err := f.Write(append(raw, '\n')); err != nil {
		return memory.Entry{}, err
	}

	return entry, nil
}

func (s *FileStore) ListAll() ([]memory.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensurePath(); err != nil {
		return nil, err
	}

	entries, _, err := s.readAllLocked()
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *FileStore) ListSince(sequence int64) ([]memory.Entry, error) {
	entries, err := s.ListAll()
	if err != nil {
		return nil, err
	}

	out := make([]memory.Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.Sequence > sequence {
			out = append(out, entry)
		}
	}

	return out, nil
}

func (s *FileStore) LatestSequence() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensurePath(); err != nil {
		return 0, err
	}

	_, maxSeq, err := s.readAllLocked()
	if err != nil {
		return 0, err
	}
	return maxSeq, nil
}

func (s *FileStore) initLocked() error {
	if s.initialized {
		return nil
	}

	_, maxSeq, err := s.readAllLocked()
	if err != nil {
		return err
	}

	s.nextSeq = maxSeq + 1
	s.initialized = true
	return nil
}

func (s *FileStore) readAllLocked() ([]memory.Entry, int64, error) {
	f, err := os.OpenFile(s.path, os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	entries := make([]memory.Entry, 0)
	var maxSeq int64

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry memory.Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, 0, err
		}

		if entry.Sequence <= 0 {
			entry.Sequence = maxSeq + 1
		}
		if entry.Sequence <= maxSeq {
			entry.Sequence = maxSeq + 1
		}
		maxSeq = entry.Sequence

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}

	return entries, maxSeq, nil
}

var _ MemoryStore = (*FileStore)(nil)
