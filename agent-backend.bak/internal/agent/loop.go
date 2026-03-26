package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"worldagent/agent-backend/internal/connectors"
	"worldagent/agent-backend/internal/extensions"
	"worldagent/agent-backend/internal/llm"
	"worldagent/agent-backend/internal/observability"
	"worldagent/agent-backend/internal/store"
	"worldagent/agent-backend/pkg/agentloop"
	"worldagent/agent-backend/pkg/ai"
)

type extensionRegistry interface {
	IsEnabled(id string) bool
}

type Runtime struct {
	store             store.MemoryStore
	registry          extensionRegistry
	audit             observability.AuditSink
	connectorRegistry *connectors.Registry
	responder         llm.Responder
	llmConnectorID    string
	logPayloads       bool
}

type RuntimeOption func(*Runtime)

func WithAuditSink(audit observability.AuditSink) RuntimeOption {
	return func(runtime *Runtime) {
		if audit != nil {
			runtime.audit = audit
		}
	}
}

func WithConnectorRegistry(registry *connectors.Registry) RuntimeOption {
	return func(runtime *Runtime) {
		runtime.connectorRegistry = registry
	}
}

func WithResponder(responder llm.Responder) RuntimeOption {
	return func(runtime *Runtime) {
		runtime.responder = responder
	}
}

func WithLLMConnectorID(connectorID string) RuntimeOption {
	return func(runtime *Runtime) {
		runtime.llmConnectorID = strings.TrimSpace(strings.ToLower(connectorID))
	}
}

func WithLogPayloads(enabled bool) RuntimeOption {
	return func(runtime *Runtime) {
		runtime.logPayloads = enabled
	}
}

type Step struct {
	Name   string `json:"name"`
	Detail string `json:"detail"`
}

type Result struct {
	Reply string `json:"reply"`
	Steps []Step `json:"steps"`
}

// NewRuntime builds the runtime integration layer that wires connectors,
// extension gating, observability, and the evented loop engine.
func NewRuntime(memoryStore store.MemoryStore, registry extensionRegistry, options ...RuntimeOption) Runtime {
	runtime := Runtime{
		store:    memoryStore,
		registry: registry,
		audit:    observability.NopAuditSink{},
	}
	for _, option := range options {
		option(&runtime)
	}
	if runtime.audit == nil {
		runtime.audit = observability.NopAuditSink{}
	}

	return runtime
}

func NewRuntimeWithAudit(memoryStore store.MemoryStore, registry extensionRegistry, audit observability.AuditSink, options ...RuntimeOption) Runtime {
	optionsWithAudit := append([]RuntimeOption{WithAuditSink(audit)}, options...)
	return NewRuntime(memoryStore, registry, optionsWithAudit...)
}

func (r Runtime) Run(message string, maxSteps int) (Result, error) {
	return r.RunWithContext(context.Background(), message, maxSteps)
}

// RunWithContext routes execution through the evented agentloop engine while
// preserving the existing external runtime result contract (reply + steps).
func (r Runtime) RunWithContext(ctx context.Context, message string, maxSteps int) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	msg := strings.TrimSpace(message)
	if msg == "" {
		return Result{}, fmt.Errorf("message is required")
	}
	if maxSteps <= 0 {
		maxSteps = 4
	}

	if _, err := r.store.Append("user", msg); err != nil {
		return Result{}, err
	}

	runState := newRuntimeState()
	plannedCalls := r.planToolCalls(ctx, msg, maxSteps, runState)

	loopEngine, err := r.newLoopEngine(ctx, msg, runState, plannedCalls)
	if err != nil {
		return Result{}, err
	}

	metadata := observability.MetadataFromContext(ctx)
	// Keep the API behavior stable while using the loop core internally:
	// one planning turn (tool calls) plus one synthesis turn (final reply).
	loopRequest := agentloop.RunRequest{
		Context: agentloop.LoopContext{
			AgentID:   "worldagent-runtime",
			RunID:     fmt.Sprintf("runtime-run-%d", time.Now().UnixNano()),
			RequestID: metadata.RequestID,
			UserID:    metadata.UserID,
			DeviceID:  metadata.DeviceID,
			TaskID:    metadata.TaskID,
		},
		Config: agentloop.Config{
			MaxTurns:      2,
			ToolExecution: agentloop.DefaultToolExecutionConfig(),
		},
		InitialMessage: agentloop.Message{
			Role:    agentloop.MessageRoleUser,
			Content: msg,
		},
	}

	loopResult, runErr := loopEngine.Run(ctx, loopRequest)
	if runErr != nil {
		return Result{}, mapLoopError(runErr)
	}
	if !loopResult.Completed {
		return Result{}, fmt.Errorf("agent loop did not complete")
	}
	if loopResult.FinalMessage == nil {
		return Result{}, fmt.Errorf("agent loop completed without final message")
	}

	reply := strings.TrimSpace(loopResult.FinalMessage.Content)
	if reply == "" {
		return Result{}, fmt.Errorf("assistant response is empty")
	}

	if _, err := r.store.Append("assistant", reply); err != nil {
		return Result{}, err
	}

	steps, _ := runState.snapshot()
	return Result{Reply: reply, Steps: steps}, nil
}

func (r Runtime) newLoopEngine(ctx context.Context, userMessage string, runState *runtimeState, plannedCalls []agentloop.ToolCall) (*agentloop.Engine, error) {
	assistantModule := &runtimeAssistantModule{
		runtime:     r,
		userMessage: userMessage,
		runState:    runState,
		planned:     cloneToolCalls(plannedCalls),
	}

	// Tools registered here define what the assistant module can call in-loop.
	// Additions in this list automatically flow into tool definitions exposed to
	// the assistant request contract.
	tools := []agentloop.Tool{
		agentloop.StaticTool{
			Def: ai.ToolDefinition{
				Name:        connectors.WebSearchConnectorID,
				Description: "Execute web search connector",
			},
			Handler: func(ctx context.Context, _ agentloop.ToolCall) (agentloop.ToolResult, error) {
				webSearchInput := r.auditPayload(
					map[string]any{
						"query": userMessage,
						"top_k": 5,
					},
					map[string]any{
						"query_chars": len(strings.TrimSpace(userMessage)),
						"top_k":       5,
					},
				)
				r.recordAudit(ctx, observability.AuditEvent{
					Type:     observability.EventToolAttempted,
					Tool:     "web-search",
					Message:  "Web-search connector execution requested",
					Metadata: withAuditPayloadMetadata(nil, webSearchInput, nil),
					Input:    webSearchInput,
				})

				searchResult, err := r.runWebSearch(ctx, userMessage)
				if err != nil {
					r.recordAudit(ctx, observability.AuditEvent{
						Type:     observability.EventToolFailed,
						Tool:     "web-search",
						Message:  "Web-search connector execution failed",
						Error:    err.Error(),
						Metadata: withAuditPayloadMetadata(nil, webSearchInput, nil),
						Input:    webSearchInput,
					})
					return agentloop.ToolResult{}, err
				}

				webSearchOutput := r.auditPayload(
					map[string]any{
						"query":   searchResult.Query,
						"summary": searchResult.Summary,
						"sources": searchResult.Sources,
					},
					map[string]any{
						"summary_chars": len(strings.TrimSpace(searchResult.Summary)),
						"sources_count": len(searchResult.Sources),
					},
				)
				runState.addStep(Step{Name: "web-search", Detail: "Executed web-search connector"})
				runState.addObservation(searchResult.Summary)
				r.recordAudit(ctx, observability.AuditEvent{
					Type:     observability.EventToolSucceeded,
					Tool:     "web-search",
					Message:  "Web-search connector execution succeeded",
					Metadata: withAuditPayloadMetadata(nil, webSearchInput, webSearchOutput),
					Input:    webSearchInput,
					Output:   webSearchOutput,
				})
				return agentloop.ToolResult{Output: searchResult}, nil
			},
		},
		agentloop.StaticTool{
			Def: ai.ToolDefinition{
				Name:        "email",
				Description: "Execute email connector",
			},
			Handler: func(ctx context.Context, _ agentloop.ToolCall) (agentloop.ToolResult, error) {
				accessToken := connectors.EmailAccessTokenFromContext(ctx)
				emailInput := r.auditPayload(
					map[string]any{
						"query":                 userMessage,
						"max_results":           5,
						"access_token_provided": strings.TrimSpace(accessToken) != "",
					},
					map[string]any{
						"query_chars":           len(strings.TrimSpace(userMessage)),
						"max_results":           5,
						"access_token_provided": strings.TrimSpace(accessToken) != "",
					},
				)
				r.recordAudit(ctx, observability.AuditEvent{
					Type:     observability.EventToolAttempted,
					Tool:     "email",
					Message:  "Email connector execution requested",
					Metadata: withAuditPayloadMetadata(nil, emailInput, nil),
					Input:    emailInput,
				})

				summary, messages, err := r.runEmailAction(ctx, userMessage)
				if err != nil {
					r.recordAudit(ctx, observability.AuditEvent{
						Type:     observability.EventToolFailed,
						Tool:     "email",
						Message:  "Email connector execution failed",
						Error:    err.Error(),
						Metadata: withAuditPayloadMetadata(nil, emailInput, nil),
						Input:    emailInput,
					})
					return agentloop.ToolResult{}, err
				}

				emailOutput := r.auditPayload(
					map[string]any{
						"summary":  summary,
						"messages": messages,
					},
					map[string]any{
						"summary_chars": len(strings.TrimSpace(summary)),
						"message_count": len(messages),
					},
				)
				runState.addStep(Step{Name: "email", Detail: "Executed email extension"})
				runState.addObservation(summary)
				r.recordAudit(ctx, observability.AuditEvent{
					Type:     observability.EventToolSucceeded,
					Tool:     "email",
					Message:  "Email connector execution succeeded",
					Metadata: withAuditPayloadMetadata(nil, emailInput, emailOutput),
					Input:    emailInput,
					Output:   emailOutput,
				})
				return agentloop.ToolResult{Output: map[string]any{"summary": summary, "messages": messages}}, nil
			},
		},
	}

	loopEngine, err := agentloop.NewEngine(assistantModule, tools)
	if err != nil {
		return nil, err
	}
	return loopEngine, nil
}

func (r Runtime) planToolCalls(ctx context.Context, msg string, maxSteps int, runState *runtimeState) []agentloop.ToolCall {
	planned := make([]agentloop.ToolCall, 0, 2)
	projectedSteps := runState.stepCount()
	lower := strings.ToLower(msg)

	if projectedSteps < maxSteps && strings.Contains(lower, "search") {
		webSearchInput := r.auditPayload(
			map[string]any{
				"query": msg,
				"top_k": 5,
			},
			map[string]any{
				"query_chars": len(strings.TrimSpace(msg)),
				"top_k":       5,
			},
		)
		if r.registry.IsEnabled("web-search") {
			planned = append(planned, agentloop.ToolCall{
				CallID:    "call-web-search",
				ToolName:  "web-search",
				Arguments: map[string]any{"query": msg, "top_k": 5},
			})
			projectedSteps++
		} else {
			runState.addObservation("Web search extension is currently disabled.")
			r.recordAudit(ctx, observability.AuditEvent{
				Type:     observability.EventToolAttempted,
				Tool:     "web-search",
				Message:  "Web-search connector execution requested",
				Metadata: withAuditPayloadMetadata(nil, webSearchInput, nil),
				Input:    webSearchInput,
			})
			r.recordAudit(ctx, observability.AuditEvent{
				Type:     observability.EventToolFailed,
				Tool:     "web-search",
				Message:  "Web-search connector execution skipped: extension disabled",
				Error:    "extension disabled",
				Metadata: withAuditPayloadMetadata(nil, webSearchInput, nil),
				Input:    webSearchInput,
			})
		}
	}

	if projectedSteps < maxSteps && strings.Contains(lower, "email") {
		accessToken := connectors.EmailAccessTokenFromContext(ctx)
		emailInput := r.auditPayload(
			map[string]any{
				"query":                 msg,
				"max_results":           5,
				"access_token_provided": strings.TrimSpace(accessToken) != "",
			},
			map[string]any{
				"query_chars":           len(strings.TrimSpace(msg)),
				"max_results":           5,
				"access_token_provided": strings.TrimSpace(accessToken) != "",
			},
		)
		if r.registry.IsEnabled("email") {
			planned = append(planned, agentloop.ToolCall{
				CallID:    "call-email",
				ToolName:  "email",
				Arguments: map[string]any{"query": msg, "max_results": 5},
			})
		} else {
			runState.addObservation("Email extension is disabled. Enable it in the Extensions tab.")
			r.recordAudit(ctx, observability.AuditEvent{
				Type:     observability.EventToolAttempted,
				Tool:     "email",
				Message:  "Email connector execution requested",
				Metadata: withAuditPayloadMetadata(nil, emailInput, nil),
				Input:    emailInput,
			})
			r.recordAudit(ctx, observability.AuditEvent{
				Type:     observability.EventToolFailed,
				Tool:     "email",
				Message:  "Email connector execution skipped: extension disabled",
				Error:    "extension disabled",
				Metadata: withAuditPayloadMetadata(nil, emailInput, nil),
				Input:    emailInput,
			})
		}
	}

	return planned
}

func mapLoopError(err error) error {
	if err == nil {
		return nil
	}

	if strings.HasPrefix(err.Error(), "generate assistant message: ") {
		if unwrapped := errors.Unwrap(err); unwrapped != nil {
			return unwrapped
		}
	}

	var toolErr *agentloop.ToolExecutionError
	if errors.As(err, &toolErr) && toolErr != nil {
		switch strings.ToLower(strings.TrimSpace(toolErr.Call.ToolName)) {
		case "web-search":
			return fmt.Errorf("web-search connector failed: %w", toolErr.Err)
		case "email":
			return fmt.Errorf("email connector failed: %w", toolErr.Err)
		}
	}

	return err
}

func (r Runtime) GenerateResponseWithGemini(ctx context.Context, prompt string) (string, error) {
	requestMetadata := map[string]string{
		"llm_connector": r.llmConnectorID,
		"prompt_chars":  fmt.Sprintf("%d", len(prompt)),
	}
	requestInput := r.auditPayload(
		map[string]any{
			"prompt": prompt,
		},
		map[string]any{
			"prompt_chars": len(prompt),
		},
	)
	if r.logPayloads {
		requestMetadata["prompt"] = prompt
	}
	r.recordAudit(ctx, observability.AuditEvent{
		Type:     observability.EventLLMRequested,
		Tool:     "llm",
		Message:  "LLM generation requested",
		Metadata: requestMetadata,
		Input:    requestInput,
	})

	responder := r.responder
	if responder == nil && r.llmConnectorID != "" {
		connector, err := connectors.GetTextGenerationConnector(r.connectorRegistry, r.llmConnectorID)
		if err != nil {
			r.recordAudit(ctx, observability.AuditEvent{
				Type:    observability.EventLLMFailed,
				Tool:    "llm",
				Message: "LLM connector lookup failed",
				Error:   err.Error(),
			})
			return "", fmt.Errorf("resolve %s connector: %w", r.llmConnectorID, err)
		}
		responder = connector
	}

	response, err := llm.GenerateAgentResponse(ctx, responder, prompt)
	if err != nil {
		r.recordAudit(ctx, observability.AuditEvent{
			Type:    observability.EventLLMFailed,
			Tool:    "llm",
			Message: "LLM generation failed",
			Error:   err.Error(),
		})
		if r.llmConnectorID != "" {
			return "", fmt.Errorf("generate response with %s connector: %w", r.llmConnectorID, err)
		}
		return "", fmt.Errorf("generate Gemini response: %w", err)
	}

	successMetadata := map[string]string{
		"response_chars": fmt.Sprintf("%d", len(response)),
	}
	responseOutput := r.auditPayload(
		map[string]any{
			"response": response,
		},
		map[string]any{
			"response_chars": len(response),
		},
	)
	if r.logPayloads {
		successMetadata["response"] = response
	}
	r.recordAudit(ctx, observability.AuditEvent{
		Type:     observability.EventLLMSucceeded,
		Tool:     "llm",
		Message:  "LLM generation succeeded",
		Metadata: successMetadata,
		Output:   responseOutput,
	})
	return response, nil
}

func (r Runtime) runWebSearch(ctx context.Context, query string) (connectors.SearchResult, error) {
	connector, err := connectors.GetWebSearchConnector(r.connectorRegistry)
	if err != nil {
		return connectors.SearchResult{}, err
	}

	result, err := connector.Search(ctx, query, 5)
	if err != nil {
		return connectors.SearchResult{}, err
	}

	result.Summary = strings.TrimSpace(result.Summary)
	summary := result.Summary
	if summary == "" {
		return connectors.SearchResult{}, fmt.Errorf("web-search connector returned empty summary")
	}
	return result, nil
}

func (r Runtime) runEmailAction(ctx context.Context, query string) (string, []connectors.EmailMessage, error) {
	connector, err := connectors.GetEmailConnector(r.connectorRegistry, connectors.GmailConnectorID)
	if err != nil {
		return "", nil, err
	}

	accessToken := connectors.EmailAccessTokenFromContext(ctx)
	messages, err := connector.ListMessages(ctx, connectors.ListMessagesRequest{
		AccessToken: accessToken,
		Query:       query,
		MaxResults:  5,
	})
	if err != nil {
		return "", nil, err
	}

	if len(messages) == 0 {
		return "Email check complete. No matching Gmail messages were found.", messages, nil
	}

	first := messages[0]
	if first.Subject != "" {
		return fmt.Sprintf("Email check complete. Found %d matching Gmail message(s). Latest subject: %q.", len(messages), first.Subject), messages, nil
	}
	return fmt.Sprintf("Email check complete. Found %d matching Gmail message(s). Latest message id: %q.", len(messages), first.ID), messages, nil
}

func (r Runtime) recordAudit(ctx context.Context, event observability.AuditEvent) {
	base := observability.EventFromContext(ctx, event.Type)
	base.Tool = event.Tool
	base.Message = event.Message
	base.Error = event.Error
	base.Metadata = event.Metadata
	base.Input = event.Input
	base.Output = event.Output
	_ = r.audit.Record(ctx, base)
}

func (r Runtime) auditPayload(fullPayload any, redactedPayload any) any {
	if r.logPayloads {
		return fullPayload
	}
	return redactedPayload
}

func withAuditPayloadMetadata(base map[string]string, input any, output any) map[string]string {
	size := len(base)
	if input != nil {
		size++
	}
	if output != nil {
		size++
	}
	metadata := make(map[string]string, size)
	for key, value := range base {
		metadata[key] = value
	}
	if input != nil {
		metadata["tool_input"] = marshalAuditPayload(input)
	}
	if output != nil {
		metadata["tool_output"] = marshalAuditPayload(output)
	}
	return metadata
}

func marshalAuditPayload(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(encoded)
}

func buildGeminiPrompt(userMessage string, observations []string) string {
	var builder strings.Builder
	builder.WriteString("You are WorldAgent. Use the observations to answer the user accurately and concisely.\n")
	builder.WriteString("User message: ")
	builder.WriteString(strings.TrimSpace(userMessage))
	builder.WriteString("\nObservations:\n")
	for _, observation := range observations {
		builder.WriteString("- ")
		builder.WriteString(strings.TrimSpace(observation))
		builder.WriteString("\n")
	}
	return builder.String()
}

type runtimeState struct {
	mu           sync.Mutex
	steps        []Step
	observations []string
}

func newRuntimeState() *runtimeState {
	return &runtimeState{
		steps: []Step{{Name: "ingest", Detail: "Received user message"}},
	}
}

func (s *runtimeState) addStep(step Step) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steps = append(s.steps, step)
}

func (s *runtimeState) addObservation(observation string) {
	trimmed := strings.TrimSpace(observation)
	if trimmed == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observations = append(s.observations, trimmed)
}

func (s *runtimeState) stepCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.steps)
}

func (s *runtimeState) snapshot() ([]Step, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	steps := make([]Step, len(s.steps))
	copy(steps, s.steps)
	observations := make([]string, len(s.observations))
	copy(observations, s.observations)
	return steps, observations
}

type runtimeAssistantModule struct {
	runtime     Runtime
	userMessage string
	runState    *runtimeState
	planned     []agentloop.ToolCall

	mu              sync.Mutex
	plannedEmitted  bool
	responseCreated bool
}

func (m *runtimeAssistantModule) Model() ai.ModelDescriptor {
	return ai.DefaultGeminiModel()
}

func (m *runtimeAssistantModule) Generate(ctx context.Context, request ai.GenerateRequest) (ai.Message, error) {
	if err := request.Validate(); err != nil {
		return ai.Message{}, err
	}

	m.mu.Lock()
	if !m.plannedEmitted && len(m.planned) > 0 {
		m.plannedEmitted = true
		content := make([]ai.ContentPart, 0, len(m.planned))
		for _, call := range m.planned {
			content = append(content, ai.ToolCallContent{Call: ai.ToolCall{
				ID:    call.CallID,
				Name:  call.ToolName,
				Input: call.Arguments,
			}})
		}
		m.mu.Unlock()
		return ai.Message{
			Role:    ai.RoleAssistant,
			Content: content,
		}, nil
	}
	m.mu.Unlock()

	observations := m.ensureObservations()
	reply, err := m.runtime.GenerateResponseWithGemini(ctx, buildGeminiPrompt(m.userMessage, observations))
	if err != nil {
		return ai.Message{}, err
	}

	m.mu.Lock()
	m.responseCreated = true
	m.mu.Unlock()

	return ai.Message{
		Role: ai.RoleAssistant,
		Content: []ai.ContentPart{
			ai.TextContent{Text: reply},
		},
	}, nil
}

func (m *runtimeAssistantModule) Stream(ctx context.Context, request ai.GenerateRequest) (ai.AssistantEventStream, error) {
	message, err := m.Generate(ctx, request)
	if err != nil {
		return ai.NewSliceEventStream(ai.ErrorEvent{Err: err}), nil
	}
	return ai.NewSliceEventStream(
		ai.MessageStartEvent{Model: m.Model()},
		ai.ContentDeltaEvent{Delta: message.Content[0]},
		ai.MessageCompleteEvent{Message: message, FinishReason: "stop"},
	), nil
}

func (m *runtimeAssistantModule) ensureObservations() []string {
	_, observations := m.runState.snapshot()
	if len(observations) > 0 {
		return observations
	}

	m.runState.addStep(Step{Name: "memory-note", Detail: "Stored task as memory note"})
	m.runState.addObservation("Got it. I stored that as context and I am ready for the next task.")
	_, observations = m.runState.snapshot()
	return observations
}

func cloneToolCalls(calls []agentloop.ToolCall) []agentloop.ToolCall {
	cloned := make([]agentloop.ToolCall, len(calls))
	copy(cloned, calls)
	return cloned
}

var _ extensionRegistry = (*extensions.Registry)(nil)
var _ ai.AssistantModule = (*runtimeAssistantModule)(nil)
