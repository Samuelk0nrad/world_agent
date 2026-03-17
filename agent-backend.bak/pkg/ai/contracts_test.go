package ai

import (
	"context"
	"io"
	"strings"
	"testing"
)

type testProvider struct {
	id        string
	newModule func(model ModelDescriptor) (AssistantModule, error)
}

func (p testProvider) ID() string {
	return p.id
}

func (p testProvider) NewAssistantModule(model ModelDescriptor) (AssistantModule, error) {
	return p.newModule(model)
}

type testAssistantModule struct {
	model ModelDescriptor
}

func (m testAssistantModule) Model() ModelDescriptor {
	return m.model
}

func (m testAssistantModule) Generate(context.Context, GenerateRequest) (Message, error) {
	return Message{
		Role: RoleAssistant,
		Content: []ContentPart{
			TextContent{Text: "ok"},
		},
	}, nil
}

func (m testAssistantModule) Stream(context.Context, GenerateRequest) (AssistantEventStream, error) {
	return testAssistantEventStream{}, nil
}

type testAssistantEventStream struct{}

func (testAssistantEventStream) Next(context.Context) (AssistantEvent, error) {
	return MessageCompleteEvent{}, io.EOF
}

func (testAssistantEventStream) Close() error {
	return nil
}

func TestContentPartTypes(t *testing.T) {
	t.Parallel()

	parts := []struct {
		part ContentPart
		kind ContentType
	}{
		{part: TextContent{Text: "hello"}, kind: ContentTypeText},
		{part: ToolCallContent{Call: ToolCall{ID: "call-1", Name: "web-search"}}, kind: ContentTypeToolCall},
		{part: ToolResultContent{Result: ToolResult{ToolCallID: "call-1", Name: "web-search"}}, kind: ContentTypeToolResult},
	}

	for _, testCase := range parts {
		if got := testCase.part.Type(); got != testCase.kind {
			t.Fatalf("expected content kind %q, got %q", testCase.kind, got)
		}
	}
}

func TestDefaultGeminiModelDescriptor(t *testing.T) {
	t.Parallel()

	model := DefaultGeminiModel()
	if model.ProviderID() != GeminiProviderID {
		t.Fatalf("expected provider %q, got %q", GeminiProviderID, model.ProviderID())
	}
	if model.ModelID() != GeminiDefaultModelID {
		t.Fatalf("expected model ID %q, got %q", GeminiDefaultModelID, model.ModelID())
	}
	if !model.Capabilities().SupportsTools {
		t.Fatal("expected Gemini model to support tools")
	}
	if !model.Capabilities().SupportsStreaming {
		t.Fatal("expected Gemini model to support streaming")
	}
}

func TestProviderRegistryRegistersAndResolvesModule(t *testing.T) {
	t.Parallel()

	registry := NewProviderRegistry()
	wantModel := DefaultGeminiModel()
	module := testAssistantModule{model: wantModel}

	if err := registry.Register(testProvider{
		id: " GeMiNi ",
		newModule: func(model ModelDescriptor) (AssistantModule, error) {
			if model.ModelID() != wantModel.ModelID() {
				t.Fatalf("expected model %q, got %q", wantModel.ModelID(), model.ModelID())
			}
			return module, nil
		},
	}); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	if _, ok := registry.Provider("gemini"); !ok {
		t.Fatal("expected normalized provider lookup to succeed")
	}

	if err := registry.Register(testProvider{id: "GEMINI", newModule: func(ModelDescriptor) (AssistantModule, error) { return module, nil }}); err == nil {
		t.Fatal("expected duplicate provider registration to fail")
	}

	resolved, err := registry.NewAssistantModule("gemini", wantModel)
	if err != nil {
		t.Fatalf("resolve module: %v", err)
	}
	if resolved.Model().ProviderID() != GeminiProviderID {
		t.Fatalf("expected resolved model provider %q, got %q", GeminiProviderID, resolved.Model().ProviderID())
	}

	resolvedByModelProvider, err := registry.NewAssistantModule("", wantModel)
	if err != nil {
		t.Fatalf("resolve module from model provider: %v", err)
	}
	if resolvedByModelProvider.Model().ProviderID() != GeminiProviderID {
		t.Fatalf("expected resolved model provider %q, got %q", GeminiProviderID, resolvedByModelProvider.Model().ProviderID())
	}
}

func TestAssistantEventKinds(t *testing.T) {
	t.Parallel()

	events := []struct {
		event AssistantEvent
		kind  AssistantEventKind
	}{
		{event: MessageStartEvent{}, kind: AssistantEventKindMessageStart},
		{event: ContentDeltaEvent{Delta: TextContent{Text: "hello"}}, kind: AssistantEventKindContentDelta},
		{event: ToolCallEvent{Call: ToolCall{ID: "call-1", Name: "web-search"}}, kind: AssistantEventKindToolCall},
		{event: ToolResultEvent{Result: ToolResult{ToolCallID: "call-1", Name: "web-search"}}, kind: AssistantEventKindToolResult},
		{event: MessageCompleteEvent{}, kind: AssistantEventKindMessageComplete},
		{event: UsageEvent{InputTokens: 10, OutputTokens: 5}, kind: AssistantEventKindUsage},
		{event: ErrorEvent{}, kind: AssistantEventKindError},
	}

	for _, testCase := range events {
		if got := testCase.event.Kind(); got != testCase.kind {
			t.Fatalf("expected event kind %q, got %q", testCase.kind, got)
		}
	}
}

func TestGenerateRequestValidateRequiresMessage(t *testing.T) {
	t.Parallel()

	err := (GenerateRequest{}).Validate()
	if err == nil || !strings.Contains(err.Error(), "at least one message is required") {
		t.Fatalf("expected explicit missing message error, got %v", err)
	}
}

func TestMessageValidateRejectsNilAndEmptyParts(t *testing.T) {
	t.Parallel()

	err := (Message{
		Role: RoleUser,
		Content: []ContentPart{
			nil,
			TextContent{},
		},
	}).Validate()
	if err == nil {
		t.Fatal("expected message validation error")
	}
	if !strings.Contains(err.Error(), "content[0]: content part is nil") {
		t.Fatalf("expected nil content part error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "content[1]: text content is required") {
		t.Fatalf("expected text validation error, got %q", err.Error())
	}
}

func TestNewAssistantModuleRejectsMismatchedModelProvider(t *testing.T) {
	t.Parallel()

	registry := NewProviderRegistry()
	if err := registry.Register(NewStaticProvider(
		"gemini",
		func(model ModelDescriptor) (AssistantModule, error) {
			return testAssistantModule{model: model}, nil
		},
	)); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	_, err := registry.NewAssistantModule("gemini", StaticModelDescriptor{
		Provider: "openai",
		ID:       "gpt-4o-mini",
		Name:     "gpt-4o-mini",
	})
	if err == nil || !strings.Contains(err.Error(), "does not match requested provider") {
		t.Fatalf("expected provider mismatch error, got %v", err)
	}
}
