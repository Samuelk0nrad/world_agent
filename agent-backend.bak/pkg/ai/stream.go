package ai

import (
	"context"
	"fmt"
	"io"
	"sync"
)

type AssistantEventKind string

const (
	AssistantEventKindMessageStart    AssistantEventKind = "message_start"
	AssistantEventKindContentDelta    AssistantEventKind = "content_delta"
	AssistantEventKindToolCall        AssistantEventKind = "tool_call"
	AssistantEventKindToolResult      AssistantEventKind = "tool_result"
	AssistantEventKindMessageComplete AssistantEventKind = "message_complete"
	AssistantEventKindUsage           AssistantEventKind = "usage"
	AssistantEventKindError           AssistantEventKind = "error"
)

type AssistantEvent interface {
	Kind() AssistantEventKind
}

type AssistantEventStream interface {
	Next(ctx context.Context) (AssistantEvent, error)
	Close() error
}

type SliceEventStream struct {
	mu     sync.Mutex
	events []AssistantEvent
	index  int
	closed bool
}

func NewSliceEventStream(events ...AssistantEvent) *SliceEventStream {
	cloned := make([]AssistantEvent, len(events))
	copy(cloned, events)
	return &SliceEventStream{
		events: cloned,
	}
}

func (s *SliceEventStream) Next(ctx context.Context) (AssistantEvent, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, io.EOF
	}
	if s.index >= len(s.events) {
		return nil, io.EOF
	}

	event := s.events[s.index]
	s.index++
	if event == nil {
		return nil, fmt.Errorf("assistant event %d is nil", s.index-1)
	}
	return event, nil
}

func (s *SliceEventStream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

type MessageStartEvent struct {
	Model ModelDescriptor
}

func (MessageStartEvent) Kind() AssistantEventKind {
	return AssistantEventKindMessageStart
}

type ContentDeltaEvent struct {
	Delta ContentPart
}

func (ContentDeltaEvent) Kind() AssistantEventKind {
	return AssistantEventKindContentDelta
}

type ToolCallEvent struct {
	Call ToolCall
}

func (ToolCallEvent) Kind() AssistantEventKind {
	return AssistantEventKindToolCall
}

type ToolResultEvent struct {
	Result ToolResult
}

func (ToolResultEvent) Kind() AssistantEventKind {
	return AssistantEventKindToolResult
}

type MessageCompleteEvent struct {
	Message      Message
	FinishReason string
}

func (MessageCompleteEvent) Kind() AssistantEventKind {
	return AssistantEventKindMessageComplete
}

type UsageEvent struct {
	InputTokens  int
	OutputTokens int
}

func (UsageEvent) Kind() AssistantEventKind {
	return AssistantEventKindUsage
}

type ErrorEvent struct {
	Err error
}

func (ErrorEvent) Kind() AssistantEventKind {
	return AssistantEventKindError
}

var _ AssistantEvent = MessageStartEvent{}
var _ AssistantEvent = ContentDeltaEvent{}
var _ AssistantEvent = ToolCallEvent{}
var _ AssistantEvent = ToolResultEvent{}
var _ AssistantEvent = MessageCompleteEvent{}
var _ AssistantEvent = UsageEvent{}
var _ AssistantEvent = ErrorEvent{}
var _ AssistantEventStream = (*SliceEventStream)(nil)
