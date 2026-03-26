package connectors

import (
	"context"
	"strings"
	"testing"
)

type testConnector struct {
	id string
}

func (c testConnector) ID() string {
	return c.id
}

type testTextGenerationConnector struct {
	id       string
	response string
}

func (c testTextGenerationConnector) ID() string {
	return c.id
}

func (c testTextGenerationConnector) Generate(context.Context, string) (string, error) {
	return c.response, nil
}

type testEmailConnector struct{}

func (c testEmailConnector) ID() string {
	return GmailConnectorID
}

func (c testEmailConnector) ListMessages(context.Context, ListMessagesRequest) ([]EmailMessage, error) {
	return []EmailMessage{{ID: "m1"}}, nil
}

func (c testEmailConnector) GetMessage(context.Context, GetMessageRequest) (EmailMessage, error) {
	return EmailMessage{ID: "m1"}, nil
}

func (c testEmailConnector) SendMessage(context.Context, SendMessageRequest) (SendMessageResponse, error) {
	return SendMessageResponse{ID: "m1"}, nil
}

func TestRegistryRegisterAndListIDs(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(testConnector{id: "zeta"}); err != nil {
		t.Fatalf("register zeta: %v", err)
	}
	if err := registry.Register(testConnector{id: "alpha"}); err != nil {
		t.Fatalf("register alpha: %v", err)
	}

	ids := registry.ListIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}
	if ids[0] != "alpha" || ids[1] != "zeta" {
		t.Fatalf("expected sorted IDs [alpha zeta], got %v", ids)
	}
}

func TestRegistryRejectsDuplicateID(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(testConnector{id: "web-search"}); err != nil {
		t.Fatalf("register first connector: %v", err)
	}

	err := registry.Register(testConnector{id: "web-search"})
	if err == nil {
		t.Fatal("expected duplicate registration error")
	}
}

func TestRegistryNormalizesConnectorIDs(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(testConnector{id: " GeMiNi "}); err != nil {
		t.Fatalf("register connector: %v", err)
	}

	_, ok := registry.Get("gemini")
	if !ok {
		t.Fatal("expected normalized connector lookup to succeed")
	}

	err := registry.Register(testConnector{id: "GEMINI"})
	if err == nil {
		t.Fatal("expected duplicate error for case-insensitive IDs")
	}
}

func TestGetTextGenerationConnector(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(testTextGenerationConnector{id: "GeMiNi", response: "ok"}); err != nil {
		t.Fatalf("register text generation connector: %v", err)
	}

	connector, err := GetTextGenerationConnector(registry, "gemini")
	if err != nil {
		t.Fatalf("expected connector, got error: %v", err)
	}

	response, err := connector.Generate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if response != "ok" {
		t.Fatalf("expected response ok, got %q", response)
	}
}

func TestGetTextGenerationConnectorRejectsWrongType(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(testConnector{id: "gemini"}); err != nil {
		t.Fatalf("register connector: %v", err)
	}

	_, err := GetTextGenerationConnector(registry, "gemini")
	if err == nil || !strings.Contains(err.Error(), "does not implement text generation") {
		t.Fatalf("expected text generation type error, got %v", err)
	}
}

func TestUnavailableWebSearchConnectorReturnsExplicitErrors(t *testing.T) {
	t.Parallel()

	connector := NewUnavailableWebSearchConnector(context.Canceled)
	_, err := connector.Search(context.Background(), "", 1)
	if err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("expected query required error, got %v", err)
	}

	_, err = connector.Search(context.Background(), "latest golang", 0)
	if err == nil || !strings.Contains(err.Error(), "topK must be greater than 0") {
		t.Fatalf("expected topK validation error, got %v", err)
	}

	_, err = connector.Search(context.Background(), "latest golang", 3)
	if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("expected wrapped unavailable error, got %v", err)
	}
}

func TestGetEmailConnector(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(testEmailConnector{}); err != nil {
		t.Fatalf("register email connector: %v", err)
	}

	connector, err := GetEmailConnector(registry, GmailConnectorID)
	if err != nil {
		t.Fatalf("get email connector: %v", err)
	}
	messages, err := connector.ListMessages(context.Background(), ListMessagesRequest{})
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 1 || messages[0].ID != "m1" {
		t.Fatalf("unexpected list response: %+v", messages)
	}
}
