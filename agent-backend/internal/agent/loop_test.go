package agent

import (
	"context"
	"strings"
	"testing"

	"worldagent/agent-backend/internal/connectors"
	"worldagent/agent-backend/internal/extensions"
	"worldagent/agent-backend/internal/observability"
	"worldagent/agent-backend/internal/store"
)

type fakeWebSearchConnector struct {
	summary string
	err     error
}

func (f fakeWebSearchConnector) ID() string {
	return connectors.WebSearchConnectorID
}

func (f fakeWebSearchConnector) Search(_ context.Context, _ string, _ int) (connectors.SearchResult, error) {
	if f.err != nil {
		return connectors.SearchResult{}, f.err
	}
	return connectors.SearchResult{Summary: f.summary}, nil
}

type fakeResponder struct {
	response string
	err      error
	prompt   string
}

func (f *fakeResponder) Generate(_ context.Context, prompt string) (string, error) {
	f.prompt = prompt
	if f.err != nil {
		return "", f.err
	}
	return f.response, nil
}

type fakeTextGenerationConnector struct {
	id       string
	response string
	err      error
}

func (f fakeTextGenerationConnector) ID() string {
	return f.id
}

func (f fakeTextGenerationConnector) Generate(context.Context, string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.response, nil
}

type fakeEmailConnector struct {
	listMessages []connectors.EmailMessage
	listErr      error
	lastRequest  connectors.ListMessagesRequest
}

func (f *fakeEmailConnector) ID() string {
	return connectors.GmailConnectorID
}

func (f *fakeEmailConnector) ListMessages(_ context.Context, request connectors.ListMessagesRequest) ([]connectors.EmailMessage, error) {
	f.lastRequest = request
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listMessages, nil
}

func (f *fakeEmailConnector) GetMessage(context.Context, connectors.GetMessageRequest) (connectors.EmailMessage, error) {
	return connectors.EmailMessage{}, nil
}

func (f *fakeEmailConnector) SendMessage(context.Context, connectors.SendMessageRequest) (connectors.SendMessageResponse, error) {
	return connectors.SendMessageResponse{}, nil
}

func TestRunExecutesWebSearchStep(t *testing.T) {
	t.Parallel()

	memoryStore := store.NewInMemoryStore()
	registry := extensions.NewDefaultRegistry()
	connectorRegistry := connectors.NewRegistry()
	if err := connectorRegistry.Register(fakeWebSearchConnector{summary: "Search results were collected."}); err != nil {
		t.Fatalf("register connector: %v", err)
	}
	responder := &fakeResponder{response: "Synthesized Gemini response"}
	runtime := NewRuntime(memoryStore, registry, WithConnectorRegistry(connectorRegistry), WithResponder(responder))

	result, err := runtime.Run("please search latest golang release notes", 4)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if len(result.Steps) < 2 {
		t.Fatalf("expected at least 2 steps, got %d", len(result.Steps))
	}
	if result.Steps[1].Name != "web-search" {
		t.Fatalf("expected second step web-search, got %q", result.Steps[1].Name)
	}
	if result.Reply != "Synthesized Gemini response" {
		t.Fatalf("expected Gemini reply, got %q", result.Reply)
	}
	if !strings.Contains(responder.prompt, "Search results were collected") {
		t.Fatalf("expected prompt to include connector observation, got %q", responder.prompt)
	}
}

func TestRunHandlesDisabledEmailExtension(t *testing.T) {
	t.Parallel()

	memoryStore := store.NewInMemoryStore()
	registry := extensions.NewDefaultRegistry()
	responder := &fakeResponder{response: "Email capability is currently disabled."}
	runtime := NewRuntime(memoryStore, registry, WithResponder(responder))

	result, err := runtime.Run("check my email for invoice", 4)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(responder.prompt), "email extension is disabled") {
		t.Fatalf("expected disabled email note in Gemini prompt, got %q", responder.prompt)
	}
	if !strings.Contains(strings.ToLower(result.Reply), "disabled") {
		t.Fatalf("expected disabled email hint, got %q", result.Reply)
	}
}

func TestRunUsesEmailConnectorWhenExtensionEnabled(t *testing.T) {
	t.Parallel()

	memoryStore := store.NewInMemoryStore()
	registry := extensions.NewDefaultRegistry()
	if _, err := registry.SetEnabled("email", true); err != nil {
		t.Fatalf("enable email extension: %v", err)
	}

	emailConnector := &fakeEmailConnector{
		listMessages: []connectors.EmailMessage{{ID: "msg-1", Subject: "Invoice due"}},
	}
	connectorRegistry := connectors.NewRegistry()
	if err := connectorRegistry.Register(emailConnector); err != nil {
		t.Fatalf("register email connector: %v", err)
	}

	responder := &fakeResponder{response: "Email summary ready."}
	runtime := NewRuntime(memoryStore, registry, WithConnectorRegistry(connectorRegistry), WithResponder(responder))
	ctx := connectors.WithEmailAccessToken(context.Background(), "token-123")

	result, err := runtime.RunWithContext(ctx, "check my email", 4)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if result.Reply != "Email summary ready." {
		t.Fatalf("expected responder output, got %q", result.Reply)
	}
	if emailConnector.lastRequest.AccessToken != "token-123" {
		t.Fatalf("expected access token passed to connector, got %q", emailConnector.lastRequest.AccessToken)
	}
	if !strings.Contains(responder.prompt, "Invoice due") {
		t.Fatalf("expected connector observation in prompt, got %q", responder.prompt)
	}
}

func TestRunFailsWhenEmailConnectorReturnsError(t *testing.T) {
	t.Parallel()

	memoryStore := store.NewInMemoryStore()
	registry := extensions.NewDefaultRegistry()
	if _, err := registry.SetEnabled("email", true); err != nil {
		t.Fatalf("enable email extension: %v", err)
	}

	emailConnector := &fakeEmailConnector{
		listErr: context.DeadlineExceeded,
	}
	connectorRegistry := connectors.NewRegistry()
	if err := connectorRegistry.Register(emailConnector); err != nil {
		t.Fatalf("register email connector: %v", err)
	}

	runtime := NewRuntime(memoryStore, registry, WithConnectorRegistry(connectorRegistry), WithResponder(&fakeResponder{response: "unused"}))
	_, err := runtime.RunWithContext(connectors.WithEmailAccessToken(context.Background(), "token-123"), "check email now", 4)
	if err == nil {
		t.Fatal("expected email connector failure")
	}
	if !strings.Contains(err.Error(), "email connector failed") {
		t.Fatalf("expected email connector wrapper, got %v", err)
	}
}

func TestRunRequiresMessage(t *testing.T) {
	t.Parallel()

	memoryStore := store.NewInMemoryStore()
	registry := extensions.NewDefaultRegistry()
	runtime := NewRuntime(memoryStore, registry, WithResponder(&fakeResponder{response: "ok"}))

	_, err := runtime.Run("   ", 4)
	if err == nil {
		t.Fatal("expected error for empty message")
	}
	if err.Error() != "message is required" {
		t.Fatalf("expected explicit error, got %q", err.Error())
	}
}

func TestRunFailsWithoutGeminiResponder(t *testing.T) {
	t.Parallel()

	memoryStore := store.NewInMemoryStore()
	registry := extensions.NewDefaultRegistry()
	runtime := NewRuntime(memoryStore, registry)

	_, err := runtime.Run("hello", 4)
	if err == nil {
		t.Fatal("expected missing responder error")
	}
	if !strings.Contains(err.Error(), "gemini responder is not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUsesConfiguredGeminiConnector(t *testing.T) {
	t.Parallel()

	memoryStore := store.NewInMemoryStore()
	registry := extensions.NewDefaultRegistry()
	connectorRegistry := connectors.NewRegistry()
	if err := connectorRegistry.Register(fakeTextGenerationConnector{id: connectors.GeminiConnectorID, response: "connector reply"}); err != nil {
		t.Fatalf("register connector: %v", err)
	}

	runtime := NewRuntime(memoryStore, registry, WithConnectorRegistry(connectorRegistry), WithLLMConnectorID(connectors.GeminiConnectorID))
	result, err := runtime.Run("hello", 4)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if result.Reply != "connector reply" {
		t.Fatalf("expected connector reply, got %q", result.Reply)
	}
}

func TestRunFailsWhenConfiguredGeminiConnectorMissing(t *testing.T) {
	t.Parallel()

	memoryStore := store.NewInMemoryStore()
	registry := extensions.NewDefaultRegistry()
	runtime := NewRuntime(memoryStore, registry, WithConnectorRegistry(connectors.NewRegistry()), WithLLMConnectorID(connectors.GeminiConnectorID))

	_, err := runtime.Run("hello", 4)
	if err == nil {
		t.Fatal("expected connector resolution error")
	}
	if !strings.Contains(err.Error(), `connector "gemini" is not registered`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunFailsWhenWebSearchConnectorMissing(t *testing.T) {
	t.Parallel()

	memoryStore := store.NewInMemoryStore()
	registry := extensions.NewDefaultRegistry()
	runtime := NewRuntime(memoryStore, registry, WithResponder(&fakeResponder{response: "ok"}))

	_, err := runtime.Run("search release notes", 4)
	if err == nil {
		t.Fatal("expected missing web-search connector error")
	}
	if !strings.Contains(err.Error(), "connector registry is not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRecordsToolAuditEvents(t *testing.T) {
	t.Parallel()

	memoryStore := store.NewInMemoryStore()
	registry := extensions.NewDefaultRegistry()
	connectorRegistry := connectors.NewRegistry()
	if err := connectorRegistry.Register(fakeWebSearchConnector{summary: "Web lookup done."}); err != nil {
		t.Fatalf("register connector: %v", err)
	}
	responder := &fakeResponder{response: "Done"}
	sink := observability.NewInMemoryAuditSink()
	runtime := NewRuntimeWithAudit(memoryStore, registry, sink, WithConnectorRegistry(connectorRegistry), WithResponder(responder))

	ctx := observability.WithMetadata(context.Background(), observability.Metadata{
		RequestID: "req-test-1",
		UserID:    "user-1",
		DeviceID:  "device-1",
		TaskID:    "task-1",
	})

	_, err := runtime.RunWithContext(ctx, "please search docs and check email", 5)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	events := sink.Events()
	if len(events) < 4 {
		t.Fatalf("expected at least 4 tool events, got %d", len(events))
	}

	var hasSearchAttempt, hasSearchSuccess, hasEmailAttempt, hasEmailFailed bool
	for _, event := range events {
		if event.RequestID != "req-test-1" || event.UserID != "user-1" || event.DeviceID != "device-1" || event.TaskID != "task-1" {
			t.Fatalf("expected metadata propagated, got event=%+v", event)
		}
		switch {
		case event.Type == observability.EventToolAttempted && event.Tool == "web-search":
			hasSearchAttempt = true
		case event.Type == observability.EventToolSucceeded && event.Tool == "web-search":
			hasSearchSuccess = true
		case event.Type == observability.EventToolAttempted && event.Tool == "email":
			hasEmailAttempt = true
		case event.Type == observability.EventToolFailed && event.Tool == "email":
			hasEmailFailed = true
		}
	}

	if !hasSearchAttempt || !hasSearchSuccess || !hasEmailAttempt || !hasEmailFailed {
		t.Fatalf("missing expected audit events: attempt(search)=%v success(search)=%v attempt(email)=%v failed(email)=%v", hasSearchAttempt, hasSearchSuccess, hasEmailAttempt, hasEmailFailed)
	}
}
