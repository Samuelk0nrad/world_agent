package agentloop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"worldagent/agent-backend/pkg/ai"
)

var ErrToolExecutionBlocked = errors.New("tool execution blocked")

type ToolExecutionError struct {
	Call    ToolCall
	Stage   string
	Err     error
	Blocked bool
}

func (e *ToolExecutionError) Error() string {
	if e == nil {
		return "tool execution error"
	}

	stage := strings.TrimSpace(e.Stage)
	if stage == "" {
		stage = "execution"
	}

	state := "failed"
	if e.Blocked {
		state = "blocked"
	}

	return fmt.Sprintf("tool call %q (%s) %s during %s: %v", e.Call.CallID, e.Call.ToolName, state, stage, e.Err)
}

func (e *ToolExecutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsToolExecutionBlocked(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrToolExecutionBlocked) {
		return true
	}
	var toolErr *ToolExecutionError
	return errors.As(err, &toolErr) && toolErr.Blocked
}

type Tool interface {
	Definition() ai.ToolDefinition
	Execute(ctx context.Context, call ToolCall) (ToolResult, error)
}

type ToolFunc func(ctx context.Context, call ToolCall) (ToolResult, error)

type StaticTool struct {
	Def     ai.ToolDefinition
	Handler ToolFunc
}

func (t StaticTool) Definition() ai.ToolDefinition {
	return t.Def
}

func (t StaticTool) Execute(ctx context.Context, call ToolCall) (ToolResult, error) {
	if t.Handler == nil {
		return ToolResult{}, fmt.Errorf("tool %q execute handler is nil", strings.TrimSpace(t.Def.Name))
	}
	return t.Handler(ctx, call)
}

type EngineOption func(*Engine)

func WithNow(now func() time.Time) EngineOption {
	return func(engine *Engine) {
		if now != nil {
			engine.now = now
		}
	}
}

type Engine struct {
	module ai.AssistantModule
	tools  map[string]Tool
	now    func() time.Time

	mu   sync.Mutex
	runs map[string]runState
}

type runState struct {
	Messages []ai.Message
	Turn     int
	LastSeq  int64
	Complete bool
}

type loopMessage struct {
	Loop Message
	AI   ai.Message
}

type executionState struct {
	events []Event
	seq    int64
	now    func() time.Time
}

type toolExecutionOutcome struct {
	call     ToolCall
	started  bool
	result   *ToolResult
	duration time.Duration
	err      error
}

func NewEngine(module ai.AssistantModule, tools []Tool, options ...EngineOption) (*Engine, error) {
	if module == nil {
		return nil, fmt.Errorf("assistant module is required")
	}
	if err := ai.ValidateModelDescriptor(module.Model()); err != nil {
		return nil, fmt.Errorf("assistant module model: %w", err)
	}

	toolMap := make(map[string]Tool, len(tools))
	for idx, tool := range tools {
		if tool == nil {
			return nil, fmt.Errorf("tools[%d] is nil", idx)
		}
		definition := tool.Definition()
		if err := definition.Validate(); err != nil {
			return nil, fmt.Errorf("tools[%d]: %w", idx, err)
		}
		toolName := normalizeToolName(definition.Name)
		if _, exists := toolMap[toolName]; exists {
			return nil, fmt.Errorf("tool %q already registered", definition.Name)
		}
		toolMap[toolName] = tool
	}

	engine := &Engine{
		module: module,
		tools:  toolMap,
		now: func() time.Time {
			return time.Now().UTC()
		},
		runs: map[string]runState{},
	}
	for _, option := range options {
		if option != nil {
			option(engine)
		}
	}
	return engine, nil
}

// Run starts a new loop execution and emits an ordered event stream describing
// assistant output and tool activity for each turn.
func (e *Engine) Run(ctx context.Context, request RunRequest) (RunResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := request.Validate(); err != nil {
		return RunResult{}, err
	}

	initialMessage, err := loopToAIMessage(request.InitialMessage)
	if err != nil {
		return RunResult{}, fmt.Errorf("initialMessage: %w", err)
	}
	initialMessages := []loopMessage{{
		Loop: request.InitialMessage,
		AI:   initialMessage,
	}}

	e.mu.Lock()
	if _, exists := e.runs[request.Context.RunID]; exists {
		e.mu.Unlock()
		return RunResult{}, fmt.Errorf("run %q already exists", request.Context.RunID)
	}
	e.mu.Unlock()

	state := runState{}
	loopResult, updatedState, execErr := e.execute(
		ctx,
		request.Context,
		request.Config,
		state,
		initialMessages,
		true,
	)

	e.mu.Lock()
	if execErr != nil {
		delete(e.runs, request.Context.RunID)
	} else {
		e.runs[request.Context.RunID] = updatedState
	}
	e.mu.Unlock()

	result := RunResult{
		State:        loopResult.State,
		Events:       loopResult.Events,
		Completed:    loopResult.Completed,
		FinalMessage: loopResult.FinalMessage,
	}
	return result, execErr
}

// Continue resumes an existing run from a previously returned continuation state.
func (e *Engine) Continue(ctx context.Context, request ContinueRequest) (ContinueResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := request.Validate(); err != nil {
		return ContinueResult{}, err
	}

	e.mu.Lock()
	state, exists := e.runs[request.Context.RunID]
	e.mu.Unlock()
	if !exists {
		return ContinueResult{}, fmt.Errorf("run %q not found", request.Context.RunID)
	}
	if state.Complete {
		return ContinueResult{}, fmt.Errorf("run %q is already completed", request.Context.RunID)
	}
	if state.Turn != request.State.Turn {
		return ContinueResult{}, fmt.Errorf("continuation turn mismatch: expected %d, got %d", state.Turn, request.State.Turn)
	}
	if state.LastSeq != request.State.LastEventSequence {
		return ContinueResult{}, fmt.Errorf("continuation event sequence mismatch: expected %d, got %d", state.LastSeq, request.State.LastEventSequence)
	}

	incoming := make([]loopMessage, len(request.IncomingMessages))
	for idx, message := range request.IncomingMessages {
		converted, err := loopToAIMessage(message)
		if err != nil {
			return ContinueResult{}, fmt.Errorf("incomingMessages[%d]: %w", idx, err)
		}
		incoming[idx] = loopMessage{
			Loop: message,
			AI:   converted,
		}
	}

	loopResult, updatedState, execErr := e.execute(
		ctx,
		request.Context,
		request.Config,
		state,
		incoming,
		false,
	)

	e.mu.Lock()
	e.runs[request.Context.RunID] = updatedState
	e.mu.Unlock()

	result := ContinueResult{
		State:        loopResult.State,
		Events:       loopResult.Events,
		Completed:    loopResult.Completed,
		FinalMessage: loopResult.FinalMessage,
	}
	return result, execErr
}

type loopExecutionResult struct {
	State        ContinuationState
	Events       []Event
	Completed    bool
	FinalMessage *Message
}

func (e *Engine) execute(
	ctx context.Context,
	loopContext LoopContext,
	config Config,
	state runState,
	initialIncoming []loopMessage,
	emitAgentStart bool,
) (loopExecutionResult, runState, error) {
	execState := executionState{
		events: make([]Event, 0, config.MaxTurns*8),
		seq:    state.LastSeq,
		now:    e.now,
	}

	if emitAgentStart {
		if err := execState.emit(loopContext, EventAgentStart, 0, nil, nil); err != nil {
			return buildExecutionResult(execState, state, false, nil), state, err
		}
	}

	incoming := cloneLoopMessages(initialIncoming)
	completed := false
	var finalMessage *Message

	for state.Turn < config.MaxTurns {
		state.Turn++
		turn := state.Turn

		if err := execState.emit(loopContext, EventTurnStart, turn, nil, nil); err != nil {
			return buildExecutionResult(execState, state, false, nil), state, err
		}

		for _, message := range incoming {
			state.Messages = append(state.Messages, message.AI)
			eventMessage := message.Loop
			if err := execState.emit(loopContext, EventMessageReceived, turn, &eventMessage, nil); err != nil {
				return buildExecutionResult(execState, state, false, nil), state, err
			}
		}

		assistantMessage, err := e.generateAssistantMessage(ctx, state.Messages)
		if err != nil {
			return buildExecutionResult(execState, state, false, nil), state, err
		}
		state.Messages = append(state.Messages, assistantMessage)

		loopAssistantMessage, err := aiToLoopMessage(assistantMessage)
		if err != nil {
			return buildExecutionResult(execState, state, false, nil), state, err
		}

		if err := execState.emit(loopContext, EventMessageDelta, turn, &loopAssistantMessage, nil); err != nil {
			return buildExecutionResult(execState, state, false, nil), state, err
		}
		if err := execState.emit(loopContext, EventMessageCompleted, turn, &loopAssistantMessage, nil); err != nil {
			return buildExecutionResult(execState, state, false, nil), state, err
		}

		toolCalls, err := extractToolCalls(assistantMessage)
		if err != nil {
			return buildExecutionResult(execState, state, false, nil), state, err
		}
		if len(toolCalls) == 0 {
			if err := execState.emit(loopContext, EventTurnEnd, turn, nil, nil); err != nil {
				return buildExecutionResult(execState, state, false, nil), state, err
			}
			if err := execState.emit(loopContext, EventAgentEnd, 0, nil, nil); err != nil {
				return buildExecutionResult(execState, state, false, nil), state, err
			}
			completed = true
			finalMessage = &loopAssistantMessage
			break
		}

		toolMessages, execErr := e.executeTools(ctx, &execState, loopContext, config, turn, toolCalls)
		if err := execState.emit(loopContext, EventTurnEnd, turn, nil, nil); err != nil {
			return buildExecutionResult(execState, state, false, nil), state, err
		}
		if execErr != nil {
			state.LastSeq = execState.seq
			return buildExecutionResult(execState, state, false, nil), state, execErr
		}
		incoming = cloneLoopMessages(toolMessages)
	}

	state.LastSeq = execState.seq
	state.Complete = completed
	return buildExecutionResult(execState, state, completed, finalMessage), state, nil
}

func buildExecutionResult(execState executionState, state runState, completed bool, finalMessage *Message) loopExecutionResult {
	return loopExecutionResult{
		State: ContinuationState{
			Turn:              state.Turn,
			LastEventSequence: execState.seq,
		},
		Events:       execState.events,
		Completed:    completed,
		FinalMessage: finalMessage,
	}
}

func (e *Engine) generateAssistantMessage(ctx context.Context, messages []ai.Message) (ai.Message, error) {
	request := ai.GenerateRequest{
		Messages: cloneAIMessages(messages),
		Tools:    e.sortedToolDefinitions(),
	}
	message, err := e.module.Generate(ctx, request)
	if err != nil {
		return ai.Message{}, fmt.Errorf("generate assistant message: %w", err)
	}
	if err := message.Validate(); err != nil {
		return ai.Message{}, fmt.Errorf("assistant message: %w", err)
	}
	if message.Role != ai.RoleAssistant {
		return ai.Message{}, fmt.Errorf("assistant message role must be %q, got %q", ai.RoleAssistant, message.Role)
	}
	return message, nil
}

func (e *Engine) executeTools(
	ctx context.Context,
	execState *executionState,
	loopContext LoopContext,
	config Config,
	turn int,
	calls []ToolCall,
) ([]loopMessage, error) {
	for _, call := range calls {
		toolEvent := &ToolExecutionEvent{
			Call: call,
			Mode: config.ToolExecution.Mode,
		}
		if err := execState.emit(loopContext, EventToolExecutionRequested, turn, nil, toolEvent); err != nil {
			return nil, err
		}
	}

	var outcomes []toolExecutionOutcome
	switch config.ToolExecution.Mode {
	case ToolExecutionModeSequential:
		outcomes = e.executeToolsSequentially(ctx, loopContext, config, turn, calls)
	case ToolExecutionModeParallel:
		outcomes = e.executeToolsInParallel(ctx, loopContext, config, turn, calls)
	default:
		return nil, fmt.Errorf("unsupported tool execution mode %q", config.ToolExecution.Mode)
	}

	incoming := make([]loopMessage, 0, len(calls))
	var firstErr error
	for _, outcome := range outcomes {
		if outcome.started {
			startedEvent := &ToolExecutionEvent{
				Call:     outcome.call,
				Mode:     config.ToolExecution.Mode,
				Duration: outcome.duration,
			}
			if err := execState.emit(loopContext, EventToolExecutionStarted, turn, nil, startedEvent); err != nil {
				return nil, err
			}
		}

		if outcome.err != nil {
			failedEvent := &ToolExecutionEvent{
				Call:     outcome.call,
				Mode:     config.ToolExecution.Mode,
				Error:    outcome.err.Error(),
				Duration: outcome.duration,
			}
			if err := execState.emit(loopContext, EventToolExecutionFailed, turn, nil, failedEvent); err != nil {
				return nil, err
			}
			if firstErr == nil {
				firstErr = outcome.err
			}
			continue
		}

		if outcome.result == nil {
			if firstErr != nil {
				continue
			}
			return nil, fmt.Errorf("tool call %q completed without result", outcome.call.CallID)
		}

		successEvent := &ToolExecutionEvent{
			Call:     outcome.call,
			Mode:     config.ToolExecution.Mode,
			Output:   outcome.result.Output,
			Duration: outcome.duration,
		}
		if err := execState.emit(loopContext, EventToolExecutionSucceeded, turn, nil, successEvent); err != nil {
			return nil, err
		}

		toolMessage, err := toolResultToMessages(outcome.call, *outcome.result)
		if err != nil {
			return nil, err
		}
		incoming = append(incoming, toolMessage)
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return incoming, nil
}

func (e *Engine) executeToolsSequentially(ctx context.Context, loopContext LoopContext, config Config, turn int, calls []ToolCall) []toolExecutionOutcome {
	outcomes := make([]toolExecutionOutcome, 0, len(calls))
	for _, call := range calls {
		outcome := e.executeSingleTool(ctx, loopContext, config, turn, call)
		outcomes = append(outcomes, outcome)
		if outcome.err != nil {
			return outcomes
		}
	}
	return outcomes
}

func (e *Engine) executeToolsInParallel(ctx context.Context, loopContext LoopContext, config Config, turn int, calls []ToolCall) []toolExecutionOutcome {
	outcomes := make([]toolExecutionOutcome, len(calls))
	for idx, call := range calls {
		outcomes[idx].call = call
	}

	workerCount := config.ToolExecution.MaxParallelTools
	if workerCount > len(calls) {
		workerCount = len(calls)
	}
	if workerCount <= 0 {
		workerCount = 1
	}

	callCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int)
	var wg sync.WaitGroup
	for idx := 0; idx < workerCount; idx++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-callCtx.Done():
					return
				case callIndex, ok := <-jobs:
					if !ok {
						return
					}
					outcome := e.executeSingleTool(callCtx, loopContext, config, turn, calls[callIndex])
					outcomes[callIndex] = outcome
					if outcome.err != nil {
						cancel()
					}
				}
			}
		}()
	}

enqueue:
	for idx := range calls {
		select {
		case <-callCtx.Done():
			break enqueue
		case jobs <- idx:
		}
	}
	close(jobs)
	wg.Wait()
	return outcomes
}

func (e *Engine) executeSingleTool(ctx context.Context, loopContext LoopContext, config Config, turn int, call ToolCall) toolExecutionOutcome {
	outcome := toolExecutionOutcome{call: call}

	beforeErr := e.runBeforeHooks(ctx, loopContext, config, turn, call)
	if beforeErr != nil {
		outcome.err = beforeErr
		if afterErr := e.runAfterHooks(ctx, loopContext, config, turn, call, nil, beforeErr, 0); afterErr != nil {
			outcome.err = afterErr
		}
		return outcome
	}

	tool, ok := e.tools[normalizeToolName(call.ToolName)]
	if !ok {
		execErr := &ToolExecutionError{
			Call:  call,
			Stage: "tool_lookup",
			Err:   fmt.Errorf("tool %q is not registered", call.ToolName),
		}
		outcome.err = execErr
		if afterErr := e.runAfterHooks(ctx, loopContext, config, turn, call, nil, execErr, 0); afterErr != nil {
			outcome.err = afterErr
		}
		return outcome
	}

	outcome.started = true
	startedAt := e.now()
	result, err := tool.Execute(ctx, call)
	outcome.duration = e.now().Sub(startedAt)

	if err != nil {
		execErr := &ToolExecutionError{
			Call:    call,
			Stage:   "tool_execute",
			Err:     err,
			Blocked: errors.Is(err, ErrToolExecutionBlocked),
		}
		outcome.err = execErr
		if afterErr := e.runAfterHooks(ctx, loopContext, config, turn, call, nil, execErr, outcome.duration); afterErr != nil {
			outcome.err = afterErr
		}
		return outcome
	}

	outcome.result = &result
	if afterErr := e.runAfterHooks(ctx, loopContext, config, turn, call, outcome.result, nil, outcome.duration); afterErr != nil {
		outcome.err = afterErr
		outcome.result = nil
	}
	return outcome
}

func (e *Engine) runBeforeHooks(ctx context.Context, loopContext LoopContext, config Config, turn int, call ToolCall) error {
	for _, hook := range config.BeforeToolHooks {
		err := hook.BeforeToolExecution(ctx, BeforeToolHookInput{
			Context: loopContext,
			Turn:    turn,
			Mode:    config.ToolExecution.Mode,
			Call:    call,
		})
		if err == nil {
			continue
		}
		return &ToolExecutionError{
			Call:    call,
			Stage:   "before_hook",
			Err:     err,
			Blocked: errors.Is(err, ErrToolExecutionBlocked),
		}
	}
	return nil
}

func (e *Engine) runAfterHooks(
	ctx context.Context,
	loopContext LoopContext,
	config Config,
	turn int,
	call ToolCall,
	result *ToolResult,
	execErr error,
	duration time.Duration,
) error {
	var hookErr error
	for _, hook := range config.AfterToolHooks {
		err := hook.AfterToolExecution(ctx, AfterToolHookInput{
			Context:  loopContext,
			Turn:     turn,
			Mode:     config.ToolExecution.Mode,
			Call:     call,
			Result:   result,
			Err:      execErr,
			Duration: duration,
		})
		if err == nil {
			continue
		}
		hookErr = &ToolExecutionError{
			Call:    call,
			Stage:   "after_hook",
			Err:     err,
			Blocked: errors.Is(err, ErrToolExecutionBlocked),
		}
		break
	}
	if hookErr == nil {
		return execErr
	}
	if execErr == nil {
		return hookErr
	}
	return errors.Join(execErr, hookErr)
}

func (e *Engine) sortedToolDefinitions() []ai.ToolDefinition {
	names := make([]string, 0, len(e.tools))
	for name := range e.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	definitions := make([]ai.ToolDefinition, 0, len(names))
	for _, name := range names {
		definitions = append(definitions, e.tools[name].Definition())
	}
	return definitions
}

func (s *executionState) emit(
	loopContext LoopContext,
	eventType EventType,
	turn int,
	message *Message,
	tool *ToolExecutionEvent,
) error {
	s.seq++
	metadata := map[string]string{
		"sequence": strconv.FormatInt(s.seq, 10),
	}
	event := Event{
		Type:          eventType,
		Timestamp:     s.now().UTC(),
		Context:       loopContext,
		Turn:          turn,
		Message:       message,
		ToolExecution: tool,
		Metadata:      metadata,
	}
	if err := event.Validate(); err != nil {
		return err
	}
	s.events = append(s.events, event)
	return nil
}

func normalizeToolName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func loopToAIMessage(message Message) (ai.Message, error) {
	if err := message.Validate(); err != nil {
		return ai.Message{}, err
	}
	role, err := toAIRole(message.Role)
	if err != nil {
		return ai.Message{}, err
	}
	return ai.Message{
		Role: role,
		Content: []ai.ContentPart{
			ai.TextContent{Text: message.Content},
		},
		Name: message.Name,
	}, nil
}

func aiToLoopMessage(message ai.Message) (Message, error) {
	if err := message.Validate(); err != nil {
		return Message{}, err
	}
	role, err := toLoopRole(message.Role)
	if err != nil {
		return Message{}, err
	}
	content, err := renderAIContent(message.Content)
	if err != nil {
		return Message{}, err
	}
	return Message{
		Role:    role,
		Content: content,
		Name:    strings.TrimSpace(message.Name),
	}, nil
}

func renderAIContent(parts []ai.ContentPart) (string, error) {
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		switch typed := part.(type) {
		case ai.TextContent:
			segments = append(segments, strings.TrimSpace(typed.Text))
		case *ai.TextContent:
			if typed != nil {
				segments = append(segments, strings.TrimSpace(typed.Text))
			}
		case ai.ToolCallContent:
			segments = append(segments, fmt.Sprintf("tool_call[%s]: %s", typed.Call.ID, typed.Call.Name))
		case *ai.ToolCallContent:
			if typed != nil {
				segments = append(segments, fmt.Sprintf("tool_call[%s]: %s", typed.Call.ID, typed.Call.Name))
			}
		case ai.ToolResultContent:
			segments = append(segments, fmt.Sprintf("tool_result[%s]: %s", typed.Result.ToolCallID, typed.Result.Name))
		case *ai.ToolResultContent:
			if typed != nil {
				segments = append(segments, fmt.Sprintf("tool_result[%s]: %s", typed.Result.ToolCallID, typed.Result.Name))
			}
		default:
			segments = append(segments, fmt.Sprintf("%v", part))
		}
	}

	combined := strings.TrimSpace(strings.Join(segments, "\n"))
	if combined == "" {
		return "", fmt.Errorf("assistant message content is empty")
	}
	return combined, nil
}

func extractToolCalls(message ai.Message) ([]ToolCall, error) {
	calls := make([]ToolCall, 0)
	for idx, part := range message.Content {
		switch typed := part.(type) {
		case ai.ToolCallContent:
			call := ToolCall{
				CallID:    typed.Call.ID,
				ToolName:  typed.Call.Name,
				Arguments: typed.Call.Input,
			}
			if err := call.Validate(); err != nil {
				return nil, fmt.Errorf("assistant tool call[%d]: %w", idx, err)
			}
			calls = append(calls, call)
		case *ai.ToolCallContent:
			if typed == nil {
				return nil, fmt.Errorf("assistant tool call[%d] is nil", idx)
			}
			call := ToolCall{
				CallID:    typed.Call.ID,
				ToolName:  typed.Call.Name,
				Arguments: typed.Call.Input,
			}
			if err := call.Validate(); err != nil {
				return nil, fmt.Errorf("assistant tool call[%d]: %w", idx, err)
			}
			calls = append(calls, call)
		}
	}
	return calls, nil
}

func toolResultToMessages(call ToolCall, result ToolResult) (loopMessage, error) {
	outputText := stringifyAny(result.Output)
	loopMsg := Message{
		Role:    MessageRoleTool,
		Content: outputText,
		Name:    call.ToolName,
	}
	if err := loopMsg.Validate(); err != nil {
		return loopMessage{}, err
	}

	aiMsg := ai.Message{
		Role:       ai.RoleTool,
		Name:       call.ToolName,
		ToolCallID: call.CallID,
		Content: []ai.ContentPart{
			ai.ToolResultContent{
				Result: ai.ToolResult{
					ToolCallID: call.CallID,
					Name:       call.ToolName,
					Output:     result.Output,
				},
			},
		},
	}
	if err := aiMsg.Validate(); err != nil {
		return loopMessage{}, err
	}
	return loopMessage{
		Loop: loopMsg,
		AI:   aiMsg,
	}, nil
}

func stringifyAny(value any) string {
	if value == nil {
		return "null"
	}
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed != "" {
			return trimmed
		}
		return `""`
	case []byte:
		trimmed := strings.TrimSpace(string(typed))
		if trimmed != "" {
			return trimmed
		}
		return `""`
	}
	encoded, err := json.Marshal(value)
	if err == nil {
		return string(encoded)
	}
	return fmt.Sprintf("%v", value)
}

func toAIRole(role MessageRole) (ai.Role, error) {
	switch role {
	case MessageRoleSystem:
		return ai.RoleSystem, nil
	case MessageRoleUser:
		return ai.RoleUser, nil
	case MessageRoleAssistant:
		return ai.RoleAssistant, nil
	case MessageRoleTool:
		return ai.RoleTool, nil
	default:
		return "", fmt.Errorf("unsupported message role %q", role)
	}
}

func toLoopRole(role ai.Role) (MessageRole, error) {
	switch role {
	case ai.RoleSystem:
		return MessageRoleSystem, nil
	case ai.RoleUser:
		return MessageRoleUser, nil
	case ai.RoleAssistant:
		return MessageRoleAssistant, nil
	case ai.RoleTool:
		return MessageRoleTool, nil
	default:
		return "", fmt.Errorf("unsupported ai role %q", role)
	}
}

func cloneAIMessages(messages []ai.Message) []ai.Message {
	cloned := make([]ai.Message, len(messages))
	copy(cloned, messages)
	return cloned
}

func cloneLoopMessages(messages []loopMessage) []loopMessage {
	cloned := make([]loopMessage, len(messages))
	copy(cloned, messages)
	return cloned
}

var _ Runner = (*Engine)(nil)
var _ Tool = StaticTool{}
