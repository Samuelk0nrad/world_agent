package agent

import (
	"context"
	"fmt"
	"strings"

	"worldagent/agent-backend/internal/connectors"
	"worldagent/agent-backend/internal/extensions"
	"worldagent/agent-backend/internal/llm"
	"worldagent/agent-backend/internal/observability"
	"worldagent/agent-backend/internal/store"
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

type Step struct {
	Name   string `json:"name"`
	Detail string `json:"detail"`
}

type Result struct {
	Reply string `json:"reply"`
	Steps []Step `json:"steps"`
}

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

	steps := []Step{{Name: "ingest", Detail: "Received user message"}}
	if _, err := r.store.Append("user", msg); err != nil {
		return Result{}, err
	}

	lower := strings.ToLower(msg)
	observations := make([]string, 0, 3)

	if len(steps) < maxSteps && strings.Contains(lower, "search") {
		r.recordAudit(ctx, observability.AuditEvent{Type: observability.EventToolAttempted, Tool: "web-search", Message: msg})
		if r.registry.IsEnabled("web-search") {
			summary, err := r.runWebSearch(ctx, msg)
			if err != nil {
				r.recordAudit(ctx, observability.AuditEvent{Type: observability.EventToolFailed, Tool: "web-search", Message: msg, Error: err.Error()})
				return Result{}, fmt.Errorf("web-search connector failed: %w", err)
			}
			steps = append(steps, Step{Name: "web-search", Detail: "Executed web-search connector"})
			observations = append(observations, summary)
			r.recordAudit(ctx, observability.AuditEvent{Type: observability.EventToolSucceeded, Tool: "web-search", Message: msg})
		} else {
			observations = append(observations, "Web search extension is currently disabled.")
			r.recordAudit(ctx, observability.AuditEvent{Type: observability.EventToolFailed, Tool: "web-search", Message: msg, Error: "extension disabled"})
		}
	}

	if len(steps) < maxSteps && strings.Contains(lower, "email") {
		r.recordAudit(ctx, observability.AuditEvent{Type: observability.EventToolAttempted, Tool: "email", Message: msg})
		if r.registry.IsEnabled("email") {
			summary, err := r.runEmailAction(ctx, msg)
			if err != nil {
				r.recordAudit(ctx, observability.AuditEvent{Type: observability.EventToolFailed, Tool: "email", Message: msg, Error: err.Error()})
				return Result{}, fmt.Errorf("email connector failed: %w", err)
			}
			steps = append(steps, Step{Name: "email", Detail: "Executed email extension"})
			observations = append(observations, summary)
			r.recordAudit(ctx, observability.AuditEvent{Type: observability.EventToolSucceeded, Tool: "email", Message: msg})
		} else {
			observations = append(observations, "Email extension is disabled. Enable it in the Extensions tab.")
			r.recordAudit(ctx, observability.AuditEvent{Type: observability.EventToolFailed, Tool: "email", Message: msg, Error: "extension disabled"})
		}
	}

	if len(observations) == 0 {
		steps = append(steps, Step{Name: "memory-note", Detail: "Stored task as memory note"})
		observations = append(observations, "Got it. I stored that as context and I am ready for the next task.")
	}

	reply, err := r.GenerateResponseWithGemini(ctx, buildGeminiPrompt(msg, observations))
	if err != nil {
		return Result{}, err
	}
	if _, err := r.store.Append("assistant", reply); err != nil {
		return Result{}, err
	}

	return Result{Reply: reply, Steps: steps}, nil
}

func (r Runtime) GenerateResponseWithGemini(ctx context.Context, prompt string) (string, error) {
	responder := r.responder
	if responder == nil && r.llmConnectorID != "" {
		connector, err := connectors.GetTextGenerationConnector(r.connectorRegistry, r.llmConnectorID)
		if err != nil {
			return "", fmt.Errorf("resolve %s connector: %w", r.llmConnectorID, err)
		}
		responder = connector
	}

	response, err := llm.GenerateAgentResponse(ctx, responder, prompt)
	if err != nil {
		if r.llmConnectorID != "" {
			return "", fmt.Errorf("generate response with %s connector: %w", r.llmConnectorID, err)
		}
		return "", fmt.Errorf("generate Gemini response: %w", err)
	}
	return response, nil
}

func (r Runtime) runWebSearch(ctx context.Context, query string) (string, error) {
	connector, err := connectors.GetWebSearchConnector(r.connectorRegistry)
	if err != nil {
		return "", err
	}

	result, err := connector.Search(ctx, query, 5)
	if err != nil {
		return "", err
	}

	summary := strings.TrimSpace(result.Summary)
	if summary == "" {
		return "", fmt.Errorf("web-search connector returned empty summary")
	}
	return summary, nil
}

func (r Runtime) runEmailAction(ctx context.Context, query string) (string, error) {
	connector, err := connectors.GetEmailConnector(r.connectorRegistry, connectors.GmailConnectorID)
	if err != nil {
		return "", err
	}

	accessToken := connectors.EmailAccessTokenFromContext(ctx)
	messages, err := connector.ListMessages(ctx, connectors.ListMessagesRequest{
		AccessToken: accessToken,
		Query:       query,
		MaxResults:  5,
	})
	if err != nil {
		return "", err
	}

	if len(messages) == 0 {
		return "Email check complete. No matching Gmail messages were found.", nil
	}

	first := messages[0]
	if first.Subject != "" {
		return fmt.Sprintf("Email check complete. Found %d matching Gmail message(s). Latest subject: %q.", len(messages), first.Subject), nil
	}
	return fmt.Sprintf("Email check complete. Found %d matching Gmail message(s). Latest message id: %q.", len(messages), first.ID), nil
}

func (r Runtime) recordAudit(ctx context.Context, event observability.AuditEvent) {
	base := observability.EventFromContext(ctx, event.Type)
	base.Tool = event.Tool
	base.Message = event.Message
	base.Error = event.Error
	base.Metadata = event.Metadata
	_ = r.audit.Record(ctx, base)
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
	builder.WriteString("Return only the final assistant response.")
	return builder.String()
}

var _ extensionRegistry = (*extensions.Registry)(nil)
