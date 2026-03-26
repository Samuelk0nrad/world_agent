// Package agentloop defines the evented agent loop contracts.
//
// The loop core exposes typed events for turns, messages, and tool execution,
// and supports continuation via ContinuationState for resume/poll style runtimes.
package agentloop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const DefaultMaxTurns = 8

type LoopContext struct {
	AgentID   string `json:"agentId"`
	RunID     string `json:"runId"`
	SessionID string `json:"sessionId,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	UserID    string `json:"userId,omitempty"`
	DeviceID  string `json:"deviceId,omitempty"`
	TaskID    string `json:"taskId,omitempty"`
}

func (c LoopContext) Validate() error {
	var validationErrors []error
	if strings.TrimSpace(c.AgentID) == "" {
		validationErrors = append(validationErrors, fmt.Errorf("agentId is required"))
	}
	if strings.TrimSpace(c.RunID) == "" {
		validationErrors = append(validationErrors, fmt.Errorf("runId is required"))
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return errors.Join(validationErrors...)
}

type ToolExecutionMode string

const (
	ToolExecutionModeSequential ToolExecutionMode = "sequential"
	ToolExecutionModeParallel   ToolExecutionMode = "parallel"
)

func (m ToolExecutionMode) Validate() error {
	switch m {
	case ToolExecutionModeSequential, ToolExecutionModeParallel:
		return nil
	case "":
		return fmt.Errorf("tool execution mode is required")
	default:
		return fmt.Errorf("unsupported tool execution mode %q", m)
	}
}

type ToolExecutionConfig struct {
	Mode             ToolExecutionMode `json:"mode"`
	MaxParallelTools int               `json:"maxParallelTools,omitempty"`
}

func DefaultToolExecutionConfig() ToolExecutionConfig {
	return ToolExecutionConfig{
		Mode:             ToolExecutionModeSequential,
		MaxParallelTools: 1,
	}
}

func (c ToolExecutionConfig) Validate() error {
	if err := c.Mode.Validate(); err != nil {
		return err
	}

	switch c.Mode {
	case ToolExecutionModeSequential:
		if c.MaxParallelTools <= 0 {
			return fmt.Errorf("maxParallelTools must be 1 when mode is sequential")
		}
		if c.MaxParallelTools != 1 {
			return fmt.Errorf("maxParallelTools must be 1 when mode is sequential")
		}
	case ToolExecutionModeParallel:
		if c.MaxParallelTools <= 1 {
			return fmt.Errorf("maxParallelTools must be greater than 1 when mode is parallel")
		}
	}

	return nil
}

type ToolCall struct {
	CallID    string `json:"callId"`
	ToolName  string `json:"toolName"`
	Arguments any    `json:"arguments,omitempty"`
}

func (c ToolCall) Validate() error {
	var validationErrors []error
	if strings.TrimSpace(c.CallID) == "" {
		validationErrors = append(validationErrors, fmt.Errorf("callId is required"))
	}
	if strings.TrimSpace(c.ToolName) == "" {
		validationErrors = append(validationErrors, fmt.Errorf("toolName is required"))
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return errors.Join(validationErrors...)
}

type ToolResult struct {
	Output   any               `json:"output,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type BeforeToolHookInput struct {
	Context LoopContext       `json:"context"`
	Turn    int               `json:"turn"`
	Mode    ToolExecutionMode `json:"mode"`
	Call    ToolCall          `json:"call"`
}

type AfterToolHookInput struct {
	Context  LoopContext       `json:"context"`
	Turn     int               `json:"turn"`
	Mode     ToolExecutionMode `json:"mode"`
	Call     ToolCall          `json:"call"`
	Result   *ToolResult       `json:"result,omitempty"`
	Err      error             `json:"-"`
	Duration time.Duration     `json:"duration,omitempty"`
}

type BeforeToolHook interface {
	BeforeToolExecution(ctx context.Context, input BeforeToolHookInput) error
}

type AfterToolHook interface {
	AfterToolExecution(ctx context.Context, input AfterToolHookInput) error
}

type BeforeToolHookFunc func(ctx context.Context, input BeforeToolHookInput) error

func (f BeforeToolHookFunc) BeforeToolExecution(ctx context.Context, input BeforeToolHookInput) error {
	if f == nil {
		return fmt.Errorf("before tool hook is nil")
	}
	return f(ctx, input)
}

type AfterToolHookFunc func(ctx context.Context, input AfterToolHookInput) error

func (f AfterToolHookFunc) AfterToolExecution(ctx context.Context, input AfterToolHookInput) error {
	if f == nil {
		return fmt.Errorf("after tool hook is nil")
	}
	return f(ctx, input)
}

type Config struct {
	MaxTurns        int                 `json:"maxTurns"`
	ToolExecution   ToolExecutionConfig `json:"toolExecution"`
	BeforeToolHooks []BeforeToolHook    `json:"-"`
	AfterToolHooks  []AfterToolHook     `json:"-"`
}

func DefaultConfig() Config {
	return Config{
		MaxTurns:      DefaultMaxTurns,
		ToolExecution: DefaultToolExecutionConfig(),
	}
}

func (c Config) Validate() error {
	var validationErrors []error
	if c.MaxTurns <= 0 {
		validationErrors = append(validationErrors, fmt.Errorf("maxTurns must be greater than 0"))
	}
	if err := c.ToolExecution.Validate(); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("toolExecution: %w", err))
	}
	for idx, hook := range c.BeforeToolHooks {
		if hook == nil {
			validationErrors = append(validationErrors, fmt.Errorf("beforeToolHooks[%d] is nil", idx))
		}
	}
	for idx, hook := range c.AfterToolHooks {
		if hook == nil {
			validationErrors = append(validationErrors, fmt.Errorf("afterToolHooks[%d] is nil", idx))
		}
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return errors.Join(validationErrors...)
}

type MessageRole string

const (
	MessageRoleSystem    MessageRole = "system"
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleTool      MessageRole = "tool"
)

func (r MessageRole) Validate() error {
	switch r {
	case MessageRoleSystem, MessageRoleUser, MessageRoleAssistant, MessageRoleTool:
		return nil
	case "":
		return fmt.Errorf("message role is required")
	default:
		return fmt.Errorf("unsupported message role %q", r)
	}
}

type Message struct {
	MessageID string      `json:"messageId,omitempty"`
	Role      MessageRole `json:"role"`
	Content   string      `json:"content"`
	Name      string      `json:"name,omitempty"`
}

func (m Message) Validate() error {
	var validationErrors []error
	if err := m.Role.Validate(); err != nil {
		validationErrors = append(validationErrors, err)
	}
	if strings.TrimSpace(m.Content) == "" {
		validationErrors = append(validationErrors, fmt.Errorf("message content is required"))
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return errors.Join(validationErrors...)
}

type EventType string

const (
	EventAgentStart             EventType = "agent_start"
	EventTurnStart              EventType = "turn_start"
	EventMessageReceived        EventType = "message_received"
	EventMessageDelta           EventType = "message_delta"
	EventMessageCompleted       EventType = "message_completed"
	EventToolExecutionRequested EventType = "tool_execution_requested"
	EventToolExecutionStarted   EventType = "tool_execution_started"
	EventToolExecutionSucceeded EventType = "tool_execution_succeeded"
	EventToolExecutionFailed    EventType = "tool_execution_failed"
	EventTurnEnd                EventType = "turn_end"
	EventAgentEnd               EventType = "agent_end"
)

func (t EventType) Validate() error {
	switch t {
	case EventAgentStart,
		EventTurnStart,
		EventMessageReceived,
		EventMessageDelta,
		EventMessageCompleted,
		EventToolExecutionRequested,
		EventToolExecutionStarted,
		EventToolExecutionSucceeded,
		EventToolExecutionFailed,
		EventTurnEnd,
		EventAgentEnd:
		return nil
	case "":
		return fmt.Errorf("event type is required")
	default:
		return fmt.Errorf("unsupported event type %q", t)
	}
}

type ToolExecutionEvent struct {
	Call     ToolCall          `json:"call"`
	Mode     ToolExecutionMode `json:"mode"`
	Output   any               `json:"output,omitempty"`
	Error    string            `json:"error,omitempty"`
	Duration time.Duration     `json:"duration,omitempty"`
}

func (e ToolExecutionEvent) Validate(eventType EventType) error {
	var validationErrors []error
	if err := e.Call.Validate(); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("call: %w", err))
	}
	if err := e.Mode.Validate(); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("mode: %w", err))
	}
	if eventType == EventToolExecutionFailed && strings.TrimSpace(e.Error) == "" {
		validationErrors = append(validationErrors, fmt.Errorf("error is required for tool_execution_failed events"))
	}
	if eventType == EventToolExecutionSucceeded && strings.TrimSpace(e.Error) != "" {
		validationErrors = append(validationErrors, fmt.Errorf("error must be empty for tool_execution_succeeded events"))
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return errors.Join(validationErrors...)
}

type Event struct {
	Type          EventType           `json:"type"`
	Timestamp     time.Time           `json:"timestamp"`
	Context       LoopContext         `json:"context"`
	Turn          int                 `json:"turn,omitempty"`
	Message       *Message            `json:"message,omitempty"`
	ToolExecution *ToolExecutionEvent `json:"toolExecution,omitempty"`
	Metadata      map[string]string   `json:"metadata,omitempty"`
}

func (e Event) Validate() error {
	var validationErrors []error
	if err := e.Type.Validate(); err != nil {
		validationErrors = append(validationErrors, err)
	}
	if e.Timestamp.IsZero() {
		validationErrors = append(validationErrors, fmt.Errorf("timestamp is required"))
	}
	if err := e.Context.Validate(); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("context: %w", err))
	}
	if requiresTurn(e.Type) && e.Turn <= 0 {
		validationErrors = append(validationErrors, fmt.Errorf("turn must be greater than 0 for event type %q", e.Type))
	}
	if isMessageEvent(e.Type) {
		if e.Message == nil {
			validationErrors = append(validationErrors, fmt.Errorf("message payload is required for event type %q", e.Type))
		} else if err := e.Message.Validate(); err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("message: %w", err))
		}
	} else if e.Message != nil {
		validationErrors = append(validationErrors, fmt.Errorf("message payload is only allowed for message events"))
	}
	if isToolExecutionEvent(e.Type) {
		if e.ToolExecution == nil {
			validationErrors = append(validationErrors, fmt.Errorf("toolExecution payload is required for event type %q", e.Type))
		} else if err := e.ToolExecution.Validate(e.Type); err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("toolExecution: %w", err))
		}
	} else if e.ToolExecution != nil {
		validationErrors = append(validationErrors, fmt.Errorf("toolExecution payload is only allowed for tool execution events"))
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return errors.Join(validationErrors...)
}

type ContinuationState struct {
	Turn              int   `json:"turn"`
	LastEventSequence int64 `json:"lastEventSequence,omitempty"`
}

func (s ContinuationState) Validate() error {
	if s.Turn <= 0 {
		return fmt.Errorf("turn must be greater than 0")
	}
	if s.LastEventSequence < 0 {
		return fmt.Errorf("lastEventSequence must be greater than or equal to 0")
	}
	return nil
}

type RunRequest struct {
	Context        LoopContext `json:"context"`
	Config         Config      `json:"config"`
	InitialMessage Message     `json:"initialMessage"`
}

func (r RunRequest) Validate() error {
	var validationErrors []error
	if err := r.Context.Validate(); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("context: %w", err))
	}
	if err := r.Config.Validate(); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("config: %w", err))
	}
	if err := r.InitialMessage.Validate(); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("initialMessage: %w", err))
	} else if r.InitialMessage.Role != MessageRoleUser {
		validationErrors = append(validationErrors, fmt.Errorf("initialMessage role must be %q", MessageRoleUser))
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return errors.Join(validationErrors...)
}

type ContinueRequest struct {
	Context          LoopContext       `json:"context"`
	Config           Config            `json:"config"`
	State            ContinuationState `json:"state"`
	IncomingMessages []Message         `json:"incomingMessages,omitempty"`
}

func (r ContinueRequest) Validate() error {
	var validationErrors []error
	if err := r.Context.Validate(); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("context: %w", err))
	}
	if err := r.Config.Validate(); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("config: %w", err))
	}
	if err := r.State.Validate(); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("state: %w", err))
	}
	for idx, message := range r.IncomingMessages {
		if err := message.Validate(); err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("incomingMessages[%d]: %w", idx, err))
		}
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return errors.Join(validationErrors...)
}

type RunResult struct {
	State        ContinuationState `json:"state"`
	Events       []Event           `json:"events"`
	Completed    bool              `json:"completed"`
	FinalMessage *Message          `json:"finalMessage,omitempty"`
}

type ContinueResult struct {
	State        ContinuationState `json:"state"`
	Events       []Event           `json:"events"`
	Completed    bool              `json:"completed"`
	FinalMessage *Message          `json:"finalMessage,omitempty"`
}

type Runner interface {
	Run(ctx context.Context, request RunRequest) (RunResult, error)
	Continue(ctx context.Context, request ContinueRequest) (ContinueResult, error)
}

type RunFunc func(ctx context.Context, request RunRequest) (RunResult, error)

type ContinueFunc func(ctx context.Context, request ContinueRequest) (ContinueResult, error)

func requiresTurn(eventType EventType) bool {
	switch eventType {
	case EventTurnStart,
		EventMessageReceived,
		EventMessageDelta,
		EventMessageCompleted,
		EventToolExecutionRequested,
		EventToolExecutionStarted,
		EventToolExecutionSucceeded,
		EventToolExecutionFailed,
		EventTurnEnd:
		return true
	default:
		return false
	}
}

func isMessageEvent(eventType EventType) bool {
	switch eventType {
	case EventMessageReceived, EventMessageDelta, EventMessageCompleted:
		return true
	default:
		return false
	}
}

func isToolExecutionEvent(eventType EventType) bool {
	switch eventType {
	case EventToolExecutionRequested, EventToolExecutionStarted, EventToolExecutionSucceeded, EventToolExecutionFailed:
		return true
	default:
		return false
	}
}
