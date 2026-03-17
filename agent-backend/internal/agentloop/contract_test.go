package agentloop

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfigIsValid(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected default config to be valid, got %v", err)
	}
}

func TestToolExecutionConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  ToolExecutionConfig
		errPart string
	}{
		{
			name: "sequential requires max parallel 1",
			config: ToolExecutionConfig{
				Mode:             ToolExecutionModeSequential,
				MaxParallelTools: 2,
			},
			errPart: "maxParallelTools must be 1 when mode is sequential",
		},
		{
			name: "parallel requires more than one worker",
			config: ToolExecutionConfig{
				Mode:             ToolExecutionModeParallel,
				MaxParallelTools: 1,
			},
			errPart: "maxParallelTools must be greater than 1 when mode is parallel",
		},
		{
			name: "invalid mode rejected",
			config: ToolExecutionConfig{
				Mode:             ToolExecutionMode("burst"),
				MaxParallelTools: 4,
			},
			errPart: "unsupported tool execution mode",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.config.Validate()
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.errPart) {
				t.Fatalf("expected error to contain %q, got %q", tc.errPart, err.Error())
			}
		})
	}
}

func TestConfigValidateRejectsNilHooks(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.BeforeToolHooks = []BeforeToolHook{nil}
	cfg.AfterToolHooks = []AfterToolHook{nil}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected nil hook validation error")
	}
	if !strings.Contains(err.Error(), "beforeToolHooks[0] is nil") {
		t.Fatalf("expected before hook error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "afterToolHooks[0] is nil") {
		t.Fatalf("expected after hook error, got %q", err.Error())
	}
}

func TestEventValidateRequiresMessagePayload(t *testing.T) {
	t.Parallel()

	event := Event{
		Type:      EventMessageReceived,
		Timestamp: time.Now().UTC(),
		Context: LoopContext{
			AgentID: "agent-1",
			RunID:   "run-1",
		},
		Turn: 1,
	}

	err := event.Validate()
	if err == nil {
		t.Fatal("expected message payload validation error")
	}
	if !strings.Contains(err.Error(), "message payload is required") {
		t.Fatalf("expected message payload error, got %q", err.Error())
	}
}

func TestEventValidateRequiresToolExecutionPayload(t *testing.T) {
	t.Parallel()

	event := Event{
		Type:      EventToolExecutionFailed,
		Timestamp: time.Now().UTC(),
		Context: LoopContext{
			AgentID: "agent-1",
			RunID:   "run-1",
		},
		Turn: 1,
	}

	err := event.Validate()
	if err == nil {
		t.Fatal("expected toolExecution payload validation error")
	}
	if !strings.Contains(err.Error(), "toolExecution payload is required") {
		t.Fatalf("expected tool execution payload error, got %q", err.Error())
	}
}

func TestEventValidateRejectsSuccessfulToolEventWithError(t *testing.T) {
	t.Parallel()

	event := Event{
		Type:      EventToolExecutionSucceeded,
		Timestamp: time.Now().UTC(),
		Context: LoopContext{
			AgentID: "agent-1",
			RunID:   "run-1",
		},
		Turn: 1,
		ToolExecution: &ToolExecutionEvent{
			Call: ToolCall{
				CallID:   "call-1",
				ToolName: "web-search",
			},
			Mode:  ToolExecutionModeSequential,
			Error: "unexpected failure",
		},
	}

	err := event.Validate()
	if err == nil {
		t.Fatal("expected error due to success event carrying failure")
	}
	if !strings.Contains(err.Error(), "error must be empty") {
		t.Fatalf("expected explicit success/failure mismatch, got %q", err.Error())
	}
}

func TestRunRequestValidateRequiresUserMessage(t *testing.T) {
	t.Parallel()

	request := RunRequest{
		Context: LoopContext{AgentID: "agent-1", RunID: "run-1"},
		Config:  DefaultConfig(),
		InitialMessage: Message{
			Role:    MessageRoleAssistant,
			Content: "hello",
		},
	}

	err := request.Validate()
	if err == nil {
		t.Fatal("expected user message validation error")
	}
	if !strings.Contains(err.Error(), "initialMessage role must be \"user\"") {
		t.Fatalf("expected user role error, got %q", err.Error())
	}
}

func TestContinueRequestValidateRequiresPositiveStateTurn(t *testing.T) {
	t.Parallel()

	request := ContinueRequest{
		Context: LoopContext{AgentID: "agent-1", RunID: "run-1"},
		Config:  DefaultConfig(),
		State: ContinuationState{
			Turn:              0,
			LastEventSequence: 10,
		},
	}

	err := request.Validate()
	if err == nil {
		t.Fatal("expected state validation error")
	}
	if !strings.Contains(err.Error(), "state: turn must be greater than 0") {
		t.Fatalf("expected state turn error, got %q", err.Error())
	}
}

func TestHookFunctionAdaptersRejectNilFunctions(t *testing.T) {
	t.Parallel()

	beforeErr := BeforeToolHookFunc(nil).BeforeToolExecution(context.Background(), BeforeToolHookInput{})
	if beforeErr == nil || beforeErr.Error() != "before tool hook is nil" {
		t.Fatalf("expected explicit nil before-hook error, got %v", beforeErr)
	}

	afterErr := AfterToolHookFunc(nil).AfterToolExecution(context.Background(), AfterToolHookInput{})
	if afterErr == nil || afterErr.Error() != "after tool hook is nil" {
		t.Fatalf("expected explicit nil after-hook error, got %v", afterErr)
	}
}
