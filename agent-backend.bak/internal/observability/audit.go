package observability

import (
	"context"
	"sync"
	"time"
)

type EventType string

const (
	EventAgentRunRequested EventType = "agent_run_requested"
	EventAgentRunCompleted EventType = "agent_run_completed"
	EventToolAttempted     EventType = "tool_execution_attempted"
	EventToolSucceeded     EventType = "tool_execution_succeeded"
	EventToolFailed        EventType = "tool_execution_failed"
	EventLLMRequested      EventType = "llm_generation_requested"
	EventLLMSucceeded      EventType = "llm_generation_succeeded"
	EventLLMFailed         EventType = "llm_generation_failed"
)

type AuditEvent struct {
	Sequence  int64             `json:"sequence"`
	Type      EventType         `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	RequestID string            `json:"requestId,omitempty"`
	UserID    string            `json:"userId,omitempty"`
	DeviceID  string            `json:"deviceId,omitempty"`
	TaskID    string            `json:"taskId,omitempty"`
	Tool      string            `json:"tool,omitempty"`
	Message   string            `json:"message,omitempty"`
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Input     any               `json:"input,omitempty"`
	Output    any               `json:"output,omitempty"`
}

type AuditSink interface {
	Record(ctx context.Context, event AuditEvent) error
}

type AuditEventStore interface {
	EventsSince(since int64, limit int, eventType EventType) []AuditEvent
	LatestSequence() int64
}

type NopAuditSink struct{}

func (NopAuditSink) Record(context.Context, AuditEvent) error {
	return nil
}

type InMemoryAuditSink struct {
	mu           sync.Mutex
	events       []AuditEvent
	nextSequence int64
	capacity     int
}

func NewInMemoryAuditSink(capacity int) *InMemoryAuditSink {
	if capacity <= 0 {
		capacity = 2000
	}
	return &InMemoryAuditSink{
		capacity: capacity,
	}
}

func (s *InMemoryAuditSink) Record(_ context.Context, event AuditEvent) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextSequence++
	event.Sequence = s.nextSequence
	s.events = append(s.events, event)
	if len(s.events) > s.capacity {
		overflow := len(s.events) - s.capacity
		s.events = s.events[overflow:]
	}
	return nil
}

func (s *InMemoryAuditSink) EventsSince(since int64, limit int, eventType EventType) []AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	out := make([]AuditEvent, 0, limit)
	for _, event := range s.events {
		if event.Sequence <= since {
			continue
		}
		if eventType != "" && event.Type != eventType {
			continue
		}
		out = append(out, event)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (s *InMemoryAuditSink) LatestSequence() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nextSequence
}

func EventFromContext(ctx context.Context, eventType EventType) AuditEvent {
	metadata := MetadataFromContext(ctx)
	return AuditEvent{
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		RequestID: metadata.RequestID,
		UserID:    metadata.UserID,
		DeviceID:  metadata.DeviceID,
		TaskID:    metadata.TaskID,
	}
}
