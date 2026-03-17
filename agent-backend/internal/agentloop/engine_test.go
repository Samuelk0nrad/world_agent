package agentloop

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"worldagent/agent-backend/internal/ai"
)

type scriptedAssistantModule struct {
	model ai.ModelDescriptor

	mu        sync.Mutex
	responses []ai.Message
	requests  []ai.GenerateRequest
	generate  func(request ai.GenerateRequest) (ai.Message, error)
}

func (m *scriptedAssistantModule) Model() ai.ModelDescriptor {
	return m.model
}

func (m *scriptedAssistantModule) Generate(_ context.Context, request ai.GenerateRequest) (ai.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requests = append(m.requests, request)
	if m.generate != nil {
		return m.generate(request)
	}
	if len(m.responses) == 0 {
		return ai.Message{}, fmt.Errorf("no scripted response available")
	}
	response := m.responses[0]
	m.responses = m.responses[1:]
	return response, nil
}

func (m *scriptedAssistantModule) Stream(context.Context, ai.GenerateRequest) (ai.AssistantEventStream, error) {
	return ai.NewSliceEventStream(), nil
}

func TestEngineRunEventOrderingIsDeterministic(t *testing.T) {
	t.Parallel()

	module := &scriptedAssistantModule{
		model: ai.DefaultGeminiModel(),
		responses: []ai.Message{
			{
				Role: ai.RoleAssistant,
				Content: []ai.ContentPart{
					ai.ToolCallContent{
						Call: ai.ToolCall{ID: "call-1", Name: "lookup", Input: map[string]any{"query": "status"}},
					},
				},
			},
			{
				Role: ai.RoleAssistant,
				Content: []ai.ContentPart{
					ai.TextContent{Text: "All done"},
				},
			},
		},
	}
	engine, err := NewEngine(module, []Tool{
		StaticTool{
			Def: ai.ToolDefinition{
				Name:        "lookup",
				Description: "lookup helper",
			},
			Handler: func(context.Context, ToolCall) (ToolResult, error) {
				return ToolResult{Output: map[string]any{"status": "ok"}}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	result, err := engine.Run(context.Background(), RunRequest{
		Context: LoopContext{
			AgentID: "agent-1",
			RunID:   "run-1",
		},
		Config: DefaultConfig(),
		InitialMessage: Message{
			Role:    MessageRoleUser,
			Content: "check status",
		},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !result.Completed {
		t.Fatal("expected run to complete")
	}
	if result.FinalMessage == nil || result.FinalMessage.Content != "All done" {
		t.Fatalf("expected final assistant message, got %+v", result.FinalMessage)
	}

	assertEventTypes(t, result.Events, []EventType{
		EventAgentStart,
		EventTurnStart,
		EventMessageReceived,
		EventMessageDelta,
		EventMessageCompleted,
		EventToolExecutionRequested,
		EventToolExecutionStarted,
		EventToolExecutionSucceeded,
		EventTurnEnd,
		EventTurnStart,
		EventMessageReceived,
		EventMessageDelta,
		EventMessageCompleted,
		EventTurnEnd,
		EventAgentEnd,
	})

	for idx, event := range result.Events {
		rawSequence, ok := event.Metadata["sequence"]
		if !ok {
			t.Fatalf("event %d missing sequence metadata", idx)
		}
		sequence, parseErr := strconv.ParseInt(rawSequence, 10, 64)
		if parseErr != nil {
			t.Fatalf("event %d has invalid sequence value %q: %v", idx, rawSequence, parseErr)
		}
		if sequence != int64(idx+1) {
			t.Fatalf("event %d sequence mismatch: expected %d, got %d", idx, idx+1, sequence)
		}
	}
}

func TestEngineToolExecutionModesSequentialAndParallel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		config       Config
		wantParallel bool
	}{
		{
			name: "sequential",
			config: Config{
				MaxTurns: DefaultMaxTurns,
				ToolExecution: ToolExecutionConfig{
					Mode:             ToolExecutionModeSequential,
					MaxParallelTools: 1,
				},
			},
			wantParallel: false,
		},
		{
			name: "parallel",
			config: Config{
				MaxTurns: DefaultMaxTurns,
				ToolExecution: ToolExecutionConfig{
					Mode:             ToolExecutionModeParallel,
					MaxParallelTools: 2,
				},
			},
			wantParallel: true,
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			tracker := &concurrencyTracker{}
			engine := mustNewEngine(t, &scriptedAssistantModule{
				model: ai.DefaultGeminiModel(),
				responses: []ai.Message{
					{
						Role: ai.RoleAssistant,
						Content: []ai.ContentPart{
							ai.ToolCallContent{Call: ai.ToolCall{ID: "call-1", Name: "worker"}},
							ai.ToolCallContent{Call: ai.ToolCall{ID: "call-2", Name: "worker"}},
						},
					},
					{
						Role: ai.RoleAssistant,
						Content: []ai.ContentPart{
							ai.TextContent{Text: "done"},
						},
					},
				},
			}, []Tool{
				StaticTool{
					Def: ai.ToolDefinition{
						Name:        "worker",
						Description: "worker tool",
					},
					Handler: func(_ context.Context, call ToolCall) (ToolResult, error) {
						tracker.start(call.CallID)
						defer tracker.finish()
						time.Sleep(70 * time.Millisecond)
						return ToolResult{Output: call.CallID}, nil
					},
				},
			})

			result, err := engine.Run(context.Background(), RunRequest{
				Context: LoopContext{
					AgentID: "agent-1",
					RunID:   "run-" + testCase.name,
				},
				Config: testCase.config,
				InitialMessage: Message{
					Role:    MessageRoleUser,
					Content: "run workers",
				},
			})
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if !result.Completed {
				t.Fatal("expected completed result")
			}

			if testCase.wantParallel {
				if tracker.max() < 2 {
					t.Fatalf("expected parallel tool execution, max in-flight=%d", tracker.max())
				}
			} else if tracker.max() != 1 {
				t.Fatalf("expected sequential tool execution, max in-flight=%d", tracker.max())
			}

			startedIDs := make([]string, 0, 2)
			for _, event := range result.Events {
				if event.Type != EventToolExecutionStarted {
					continue
				}
				startedIDs = append(startedIDs, event.ToolExecution.Call.CallID)
			}
			if !reflect.DeepEqual(startedIDs, []string{"call-1", "call-2"}) {
				t.Fatalf("expected deterministic started event ordering, got %v", startedIDs)
			}
		})
	}
}

func TestEngineBeforeHookCanBlockAndAfterHookObservesError(t *testing.T) {
	t.Parallel()

	toolExecuted := false
	afterCalled := false
	afterSawBlockedErr := false

	cfg := DefaultConfig()
	cfg.BeforeToolHooks = []BeforeToolHook{
		BeforeToolHookFunc(func(context.Context, BeforeToolHookInput) error {
			return fmt.Errorf("%w: denied by policy", ErrToolExecutionBlocked)
		}),
	}
	cfg.AfterToolHooks = []AfterToolHook{
		AfterToolHookFunc(func(_ context.Context, input AfterToolHookInput) error {
			afterCalled = true
			afterSawBlockedErr = IsToolExecutionBlocked(input.Err)
			return nil
		}),
	}

	engine := mustNewEngine(t, &scriptedAssistantModule{
		model: ai.DefaultGeminiModel(),
		responses: []ai.Message{
			{
				Role: ai.RoleAssistant,
				Content: []ai.ContentPart{
					ai.ToolCallContent{Call: ai.ToolCall{ID: "call-1", Name: "guarded"}},
				},
			},
		},
	}, []Tool{
		StaticTool{
			Def: ai.ToolDefinition{Name: "guarded"},
			Handler: func(context.Context, ToolCall) (ToolResult, error) {
				toolExecuted = true
				return ToolResult{Output: "never"}, nil
			},
		},
	})

	result, err := engine.Run(context.Background(), RunRequest{
		Context: LoopContext{AgentID: "agent-1", RunID: "run-blocked"},
		Config:  cfg,
		InitialMessage: Message{
			Role:    MessageRoleUser,
			Content: "do guarded task",
		},
	})
	if err == nil {
		t.Fatal("expected blocked run error")
	}
	if !IsToolExecutionBlocked(err) {
		t.Fatalf("expected blocked error, got %v", err)
	}
	if result.Completed {
		t.Fatal("expected incomplete result when blocked")
	}
	if toolExecuted {
		t.Fatal("tool should not have executed when before hook blocked")
	}
	if !afterCalled || !afterSawBlockedErr {
		t.Fatalf("expected after hook to observe blocked error, called=%v sawBlocked=%v", afterCalled, afterSawBlockedErr)
	}

	var startedCount int
	var failedCount int
	for _, event := range result.Events {
		if event.Type == EventToolExecutionStarted {
			startedCount++
		}
		if event.Type == EventToolExecutionFailed {
			failedCount++
		}
	}
	if startedCount != 0 {
		t.Fatalf("expected no started event for blocked tool, got %d", startedCount)
	}
	if failedCount != 1 {
		t.Fatalf("expected one failed tool event, got %d", failedCount)
	}
}

func TestEngineBeforeHookErrorSkipsToolAndMarksFailure(t *testing.T) {
	t.Parallel()

	toolExecuted := false
	afterCalled := false
	var afterErr error

	cfg := DefaultConfig()
	cfg.BeforeToolHooks = []BeforeToolHook{
		BeforeToolHookFunc(func(context.Context, BeforeToolHookInput) error {
			return errors.New("policy backend unavailable")
		}),
	}
	cfg.AfterToolHooks = []AfterToolHook{
		AfterToolHookFunc(func(_ context.Context, input AfterToolHookInput) error {
			afterCalled = true
			afterErr = input.Err
			return nil
		}),
	}

	engine := mustNewEngine(t, &scriptedAssistantModule{
		model: ai.DefaultGeminiModel(),
		responses: []ai.Message{
			{
				Role: ai.RoleAssistant,
				Content: []ai.ContentPart{
					ai.ToolCallContent{Call: ai.ToolCall{ID: "call-1", Name: "guarded"}},
				},
			},
		},
	}, []Tool{
		StaticTool{
			Def: ai.ToolDefinition{Name: "guarded"},
			Handler: func(context.Context, ToolCall) (ToolResult, error) {
				toolExecuted = true
				return ToolResult{Output: "never"}, nil
			},
		},
	})

	result, err := engine.Run(context.Background(), RunRequest{
		Context: LoopContext{AgentID: "agent-1", RunID: "run-before-hook-error"},
		Config:  cfg,
		InitialMessage: Message{
			Role:    MessageRoleUser,
			Content: "execute guarded tool",
		},
	})
	if err == nil {
		t.Fatal("expected before-hook run error")
	}
	if IsToolExecutionBlocked(err) {
		t.Fatalf("expected non-blocked before hook failure, got %v", err)
	}
	if result.Completed {
		t.Fatal("expected incomplete result when before hook fails")
	}
	if toolExecuted {
		t.Fatal("tool should not execute when before hook fails")
	}
	if !afterCalled {
		t.Fatal("expected after hook to run when before hook fails")
	}

	var beforeHookErr *ToolExecutionError
	if !errors.As(afterErr, &beforeHookErr) {
		t.Fatalf("expected after hook to receive tool execution error, got %v", afterErr)
	}
	if beforeHookErr.Stage != "before_hook" {
		t.Fatalf("expected before_hook stage, got %q", beforeHookErr.Stage)
	}

	startedIDs := collectToolCallIDsByEventType(result.Events, EventToolExecutionStarted)
	if len(startedIDs) != 0 {
		t.Fatalf("expected no started events, got %v", startedIDs)
	}
	failedIDs := collectToolCallIDsByEventType(result.Events, EventToolExecutionFailed)
	if !reflect.DeepEqual(failedIDs, []string{"call-1"}) {
		t.Fatalf("expected single failed call, got %v", failedIDs)
	}
}

func TestEngineHooksObserveSuccessfulToolExecution(t *testing.T) {
	t.Parallel()

	var beforeInputs []BeforeToolHookInput
	var afterInputs []AfterToolHookInput

	cfg := DefaultConfig()
	cfg.BeforeToolHooks = []BeforeToolHook{
		BeforeToolHookFunc(func(_ context.Context, input BeforeToolHookInput) error {
			beforeInputs = append(beforeInputs, input)
			return nil
		}),
	}
	cfg.AfterToolHooks = []AfterToolHook{
		AfterToolHookFunc(func(_ context.Context, input AfterToolHookInput) error {
			afterInputs = append(afterInputs, input)
			return nil
		}),
	}

	engine := mustNewEngine(t, &scriptedAssistantModule{
		model: ai.DefaultGeminiModel(),
		responses: []ai.Message{
			{
				Role: ai.RoleAssistant,
				Content: []ai.ContentPart{
					ai.ToolCallContent{Call: ai.ToolCall{ID: "call-1", Name: "worker"}},
				},
			},
			{
				Role: ai.RoleAssistant,
				Content: []ai.ContentPart{
					ai.TextContent{Text: "done"},
				},
			},
		},
	}, []Tool{
		StaticTool{
			Def: ai.ToolDefinition{Name: "worker"},
			Handler: func(context.Context, ToolCall) (ToolResult, error) {
				return ToolResult{Output: "ok"}, nil
			},
		},
	})

	result, err := engine.Run(context.Background(), RunRequest{
		Context: LoopContext{AgentID: "agent-1", RunID: "run-hook-success"},
		Config:  cfg,
		InitialMessage: Message{
			Role:    MessageRoleUser,
			Content: "run worker",
		},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !result.Completed {
		t.Fatal("expected completed run")
	}
	if len(beforeInputs) != 1 {
		t.Fatalf("expected one before-hook call, got %d", len(beforeInputs))
	}
	if len(afterInputs) != 1 {
		t.Fatalf("expected one after-hook call, got %d", len(afterInputs))
	}
	if beforeInputs[0].Call.CallID != "call-1" || beforeInputs[0].Mode != ToolExecutionModeSequential || beforeInputs[0].Turn != 1 {
		t.Fatalf("unexpected before-hook input: %+v", beforeInputs[0])
	}
	if beforeInputs[0].Context.RunID != "run-hook-success" {
		t.Fatalf("expected before-hook context run id propagated, got %+v", beforeInputs[0].Context)
	}
	if afterInputs[0].Err != nil {
		t.Fatalf("expected nil execution error in after hook, got %v", afterInputs[0].Err)
	}
	if afterInputs[0].Result == nil || afterInputs[0].Result.Output != "ok" {
		t.Fatalf("expected tool result in after hook, got %+v", afterInputs[0].Result)
	}
	if afterInputs[0].Call.CallID != "call-1" {
		t.Fatalf("unexpected after-hook call data: %+v", afterInputs[0].Call)
	}

	successIDs := collectToolCallIDsByEventType(result.Events, EventToolExecutionSucceeded)
	if !reflect.DeepEqual(successIDs, []string{"call-1"}) {
		t.Fatalf("expected one successful tool event, got %v", successIDs)
	}
}

func TestEngineAfterHookCanBlockAndMarksFailure(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.AfterToolHooks = []AfterToolHook{
		AfterToolHookFunc(func(context.Context, AfterToolHookInput) error {
			return fmt.Errorf("%w: blocked by post-policy", ErrToolExecutionBlocked)
		}),
	}

	engine := mustNewEngine(t, &scriptedAssistantModule{
		model: ai.DefaultGeminiModel(),
		responses: []ai.Message{
			{
				Role: ai.RoleAssistant,
				Content: []ai.ContentPart{
					ai.ToolCallContent{Call: ai.ToolCall{ID: "call-1", Name: "worker"}},
				},
			},
		},
	}, []Tool{
		StaticTool{
			Def: ai.ToolDefinition{Name: "worker"},
			Handler: func(context.Context, ToolCall) (ToolResult, error) {
				return ToolResult{Output: "ok"}, nil
			},
		},
	})

	result, err := engine.Run(context.Background(), RunRequest{
		Context: LoopContext{AgentID: "agent-1", RunID: "run-after-hook-block"},
		Config:  cfg,
		InitialMessage: Message{
			Role:    MessageRoleUser,
			Content: "run worker",
		},
	})
	if err == nil {
		t.Fatal("expected after-hook blocked error")
	}
	if !IsToolExecutionBlocked(err) {
		t.Fatalf("expected blocked error, got %v", err)
	}
	var toolErr *ToolExecutionError
	if !errors.As(err, &toolErr) {
		t.Fatalf("expected tool execution error type, got %T", err)
	}
	if toolErr.Stage != "after_hook" {
		t.Fatalf("expected after_hook stage, got %q", toolErr.Stage)
	}
	if result.Completed {
		t.Fatal("expected incomplete result when after hook blocks")
	}

	startedIDs := collectToolCallIDsByEventType(result.Events, EventToolExecutionStarted)
	if !reflect.DeepEqual(startedIDs, []string{"call-1"}) {
		t.Fatalf("expected started event for call-1, got %v", startedIDs)
	}
	failedIDs := collectToolCallIDsByEventType(result.Events, EventToolExecutionFailed)
	if !reflect.DeepEqual(failedIDs, []string{"call-1"}) {
		t.Fatalf("expected failed event for call-1, got %v", failedIDs)
	}
	successIDs := collectToolCallIDsByEventType(result.Events, EventToolExecutionSucceeded)
	if len(successIDs) != 0 {
		t.Fatalf("expected no success events when after hook blocks, got %v", successIDs)
	}
}

func TestEngineAfterHookCanMutateToolResult(t *testing.T) {
	t.Parallel()

	module := &scriptedAssistantModule{
		model: ai.DefaultGeminiModel(),
	}
	callCount := 0
	module.generate = func(request ai.GenerateRequest) (ai.Message, error) {
		callCount++
		if callCount == 1 {
			return ai.Message{
				Role: ai.RoleAssistant,
				Content: []ai.ContentPart{
					ai.ToolCallContent{Call: ai.ToolCall{ID: "call-1", Name: "mutating-tool"}},
				},
			}, nil
		}

		lastMessage := request.Messages[len(request.Messages)-1]
		if lastMessage.Role != ai.RoleTool {
			return ai.Message{}, fmt.Errorf("expected last message role tool, got %q", lastMessage.Role)
		}
		resultPart, ok := lastMessage.Content[0].(ai.ToolResultContent)
		if !ok {
			return ai.Message{}, fmt.Errorf("expected tool result part, got %T", lastMessage.Content[0])
		}
		if resultPart.Result.Output != "mutated" {
			return ai.Message{}, fmt.Errorf("expected mutated output, got %#v", resultPart.Result.Output)
		}
		return ai.Message{
			Role: ai.RoleAssistant,
			Content: []ai.ContentPart{
				ai.TextContent{Text: "mutation applied"},
			},
		}, nil
	}

	cfg := DefaultConfig()
	cfg.AfterToolHooks = []AfterToolHook{
		AfterToolHookFunc(func(_ context.Context, input AfterToolHookInput) error {
			if input.Result == nil {
				return errors.New("missing result")
			}
			input.Result.Output = "mutated"
			return nil
		}),
	}

	engine := mustNewEngine(t, module, []Tool{
		StaticTool{
			Def: ai.ToolDefinition{Name: "mutating-tool"},
			Handler: func(context.Context, ToolCall) (ToolResult, error) {
				return ToolResult{Output: "original"}, nil
			},
		},
	})

	result, err := engine.Run(context.Background(), RunRequest{
		Context: LoopContext{AgentID: "agent-1", RunID: "run-mutating-hook"},
		Config:  cfg,
		InitialMessage: Message{
			Role:    MessageRoleUser,
			Content: "mutate tool output",
		},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !result.Completed {
		t.Fatal("expected completed run")
	}
	if result.FinalMessage == nil || result.FinalMessage.Content != "mutation applied" {
		t.Fatalf("unexpected final message: %+v", result.FinalMessage)
	}

	var successEvent *Event
	for idx := range result.Events {
		if result.Events[idx].Type == EventToolExecutionSucceeded {
			successEvent = &result.Events[idx]
			break
		}
	}
	if successEvent == nil {
		t.Fatal("expected tool success event")
	}
	if output, ok := successEvent.ToolExecution.Output.(string); !ok || output != "mutated" {
		t.Fatalf("expected mutated tool output in event, got %#v", successEvent.ToolExecution.Output)
	}
}

func TestEngineAfterHookErrorJoinsToolExecutionError(t *testing.T) {
	t.Parallel()

	toolErr := errors.New("tool crashed")
	afterErr := errors.New("audit sink failed")

	cfg := DefaultConfig()
	cfg.AfterToolHooks = []AfterToolHook{
		AfterToolHookFunc(func(context.Context, AfterToolHookInput) error {
			return afterErr
		}),
	}

	engine := mustNewEngine(t, &scriptedAssistantModule{
		model: ai.DefaultGeminiModel(),
		responses: []ai.Message{
			{
				Role: ai.RoleAssistant,
				Content: []ai.ContentPart{
					ai.ToolCallContent{Call: ai.ToolCall{ID: "call-1", Name: "worker"}},
				},
			},
		},
	}, []Tool{
		StaticTool{
			Def: ai.ToolDefinition{Name: "worker"},
			Handler: func(context.Context, ToolCall) (ToolResult, error) {
				return ToolResult{}, toolErr
			},
		},
	})

	result, err := engine.Run(context.Background(), RunRequest{
		Context: LoopContext{AgentID: "agent-1", RunID: "run-after-hook-error"},
		Config:  cfg,
		InitialMessage: Message{
			Role:    MessageRoleUser,
			Content: "run worker",
		},
	})
	if err == nil {
		t.Fatal("expected combined execution error")
	}
	if !errors.Is(err, toolErr) {
		t.Fatalf("expected tool error to be preserved, got %v", err)
	}
	if !errors.Is(err, afterErr) {
		t.Fatalf("expected after-hook error to be preserved, got %v", err)
	}
	if result.Completed {
		t.Fatal("expected incomplete result on combined error")
	}

	failedIDs := collectToolCallIDsByEventType(result.Events, EventToolExecutionFailed)
	if !reflect.DeepEqual(failedIDs, []string{"call-1"}) {
		t.Fatalf("expected failed event for call-1, got %v", failedIDs)
	}
	successIDs := collectToolCallIDsByEventType(result.Events, EventToolExecutionSucceeded)
	if len(successIDs) != 0 {
		t.Fatalf("expected no success events, got %v", successIDs)
	}
}

func TestEngineToolFailureAbortsSequentialPipeline(t *testing.T) {
	t.Parallel()

	secondCalled := false
	engine := mustNewEngine(t, &scriptedAssistantModule{
		model: ai.DefaultGeminiModel(),
		responses: []ai.Message{
			{
				Role: ai.RoleAssistant,
				Content: []ai.ContentPart{
					ai.ToolCallContent{Call: ai.ToolCall{ID: "call-1", Name: "worker"}},
					ai.ToolCallContent{Call: ai.ToolCall{ID: "call-2", Name: "worker"}},
				},
			},
		},
	}, []Tool{
		StaticTool{
			Def: ai.ToolDefinition{Name: "worker"},
			Handler: func(_ context.Context, call ToolCall) (ToolResult, error) {
				if call.CallID == "call-1" {
					return ToolResult{}, fmt.Errorf("boom")
				}
				secondCalled = true
				return ToolResult{Output: "ok"}, nil
			},
		},
	})

	cfg := DefaultConfig()
	cfg.ToolExecution = ToolExecutionConfig{
		Mode:             ToolExecutionModeSequential,
		MaxParallelTools: 1,
	}
	result, err := engine.Run(context.Background(), RunRequest{
		Context: LoopContext{AgentID: "agent-1", RunID: "run-failure"},
		Config:  cfg,
		InitialMessage: Message{
			Role:    MessageRoleUser,
			Content: "run tools",
		},
	})
	if err == nil {
		t.Fatal("expected tool failure")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected underlying tool error, got %v", err)
	}
	if result.Completed {
		t.Fatal("expected incomplete result on tool error")
	}
	if secondCalled {
		t.Fatal("expected second tool call to be skipped after failure")
	}

	requestedIDs := collectToolCallIDsByEventType(result.Events, EventToolExecutionRequested)
	if !reflect.DeepEqual(requestedIDs, []string{"call-1", "call-2"}) {
		t.Fatalf("unexpected requested IDs: %v", requestedIDs)
	}
	startedIDs := collectToolCallIDsByEventType(result.Events, EventToolExecutionStarted)
	if !reflect.DeepEqual(startedIDs, []string{"call-1"}) {
		t.Fatalf("expected only first call started, got %v", startedIDs)
	}
	failedIDs := collectToolCallIDsByEventType(result.Events, EventToolExecutionFailed)
	if !reflect.DeepEqual(failedIDs, []string{"call-1"}) {
		t.Fatalf("expected only first call failed, got %v", failedIDs)
	}
}

func TestEngineParallelFailureCancelsSiblingToolCall(t *testing.T) {
	t.Parallel()

	failErr := errors.New("boom")
	call2Started := make(chan struct{})
	var call2StartedOnce sync.Once

	engine := mustNewEngine(t, &scriptedAssistantModule{
		model: ai.DefaultGeminiModel(),
		responses: []ai.Message{
			{
				Role: ai.RoleAssistant,
				Content: []ai.ContentPart{
					ai.ToolCallContent{Call: ai.ToolCall{ID: "call-1", Name: "worker"}},
					ai.ToolCallContent{Call: ai.ToolCall{ID: "call-2", Name: "worker"}},
				},
			},
		},
	}, []Tool{
		StaticTool{
			Def: ai.ToolDefinition{Name: "worker"},
			Handler: func(ctx context.Context, call ToolCall) (ToolResult, error) {
				switch call.CallID {
				case "call-1":
					select {
					case <-call2Started:
					case <-time.After(time.Second):
						return ToolResult{}, errors.New("timed out waiting for call-2 start")
					}
					return ToolResult{}, failErr
				case "call-2":
					call2StartedOnce.Do(func() { close(call2Started) })
					<-ctx.Done()
					return ToolResult{}, ctx.Err()
				default:
					return ToolResult{}, fmt.Errorf("unexpected call id %q", call.CallID)
				}
			},
		},
	})

	cfg := DefaultConfig()
	cfg.ToolExecution = ToolExecutionConfig{
		Mode:             ToolExecutionModeParallel,
		MaxParallelTools: 2,
	}
	result, err := engine.Run(context.Background(), RunRequest{
		Context: LoopContext{AgentID: "agent-1", RunID: "run-parallel-cancel"},
		Config:  cfg,
		InitialMessage: Message{
			Role:    MessageRoleUser,
			Content: "run workers",
		},
	})
	if err == nil {
		t.Fatal("expected parallel tool failure")
	}
	if !errors.Is(err, failErr) {
		t.Fatalf("expected primary tool error, got %v", err)
	}
	if result.Completed {
		t.Fatal("expected incomplete result")
	}

	startedIDs := collectToolCallIDsByEventType(result.Events, EventToolExecutionStarted)
	if !reflect.DeepEqual(startedIDs, []string{"call-1", "call-2"}) {
		t.Fatalf("expected both calls to start, got %v", startedIDs)
	}

	var call2Failure string
	for _, event := range result.Events {
		if event.Type == EventToolExecutionFailed && event.ToolExecution != nil && event.ToolExecution.Call.CallID == "call-2" {
			call2Failure = event.ToolExecution.Error
			break
		}
	}
	if !strings.Contains(call2Failure, context.Canceled.Error()) {
		t.Fatalf("expected call-2 cancellation error, got %q", call2Failure)
	}
}

func TestEngineRunPropagatesContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	engine := mustNewEngine(t, contextAwareAssistantModule{model: ai.DefaultGeminiModel()}, nil)
	result, err := engine.Run(ctx, RunRequest{
		Context: LoopContext{AgentID: "agent-1", RunID: "run-canceled"},
		Config:  DefaultConfig(),
		InitialMessage: Message{
			Role:    MessageRoleUser,
			Content: "hello",
		},
	})
	if err == nil {
		t.Fatal("expected canceled context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
	if result.Completed {
		t.Fatal("expected incomplete result when context is canceled")
	}
}

type contextAwareAssistantModule struct {
	model ai.ModelDescriptor
}

func (m contextAwareAssistantModule) Model() ai.ModelDescriptor {
	return m.model
}

func (m contextAwareAssistantModule) Generate(ctx context.Context, _ ai.GenerateRequest) (ai.Message, error) {
	<-ctx.Done()
	return ai.Message{}, ctx.Err()
}

func (contextAwareAssistantModule) Stream(context.Context, ai.GenerateRequest) (ai.AssistantEventStream, error) {
	return ai.NewSliceEventStream(), nil
}

type concurrencyTracker struct {
	mu          sync.Mutex
	inFlight    int
	maxInFlight int
}

func (t *concurrencyTracker) start(_ string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inFlight++
	if t.inFlight > t.maxInFlight {
		t.maxInFlight = t.inFlight
	}
}

func (t *concurrencyTracker) finish() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.inFlight--
}

func (t *concurrencyTracker) max() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.maxInFlight
}

func mustNewEngine(t *testing.T, module ai.AssistantModule, tools []Tool) *Engine {
	t.Helper()
	engine, err := NewEngine(module, tools)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return engine
}

func collectToolCallIDsByEventType(events []Event, eventType EventType) []string {
	callIDs := make([]string, 0)
	for _, event := range events {
		if event.Type != eventType || event.ToolExecution == nil {
			continue
		}
		callIDs = append(callIDs, event.ToolExecution.Call.CallID)
	}
	return callIDs
}

func assertEventTypes(t *testing.T, events []Event, want []EventType) {
	t.Helper()

	got := make([]EventType, 0, len(events))
	for _, event := range events {
		got = append(got, event.Type)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("event ordering mismatch\nwant: %v\ngot:  %v", want, got)
	}
}
