package observability

import (
	"context"
	"sync"
	"time"
)

type EventType string

const (
	EventAgentRunRequested EventType = "agent_run_requested"
	EventToolAttempted     EventType = "tool_execution_attempted"
	EventToolSucceeded     EventType = "tool_execution_succeeded"
	EventToolFailed        EventType = "tool_execution_failed"
)

type AuditEvent struct {
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
}

type AuditSink interface {
	Record(ctx context.Context, event AuditEvent) error
}

type NopAuditSink struct{}

func (NopAuditSink) Record(context.Context, AuditEvent) error {
	return nil
}

type InMemoryAuditSink struct {
	mu     sync.Mutex
	events []AuditEvent
}

func NewInMemoryAuditSink() *InMemoryAuditSink {
	return &InMemoryAuditSink{}
}

func (s *InMemoryAuditSink) Record(_ context.Context, event AuditEvent) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *InMemoryAuditSink) Events() []AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]AuditEvent, len(s.events))
	copy(out, s.events)
	return out
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
