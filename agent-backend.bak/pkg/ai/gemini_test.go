package ai

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"worldagent/agent-backend/internal/config"
)

type stubGeminiGenerator struct {
	response string
	err      error
	prompts  *[]string
}

func (g stubGeminiGenerator) Generate(_ context.Context, prompt string) (string, error) {
	if g.prompts != nil {
		*g.prompts = append(*g.prompts, prompt)
	}
	if g.err != nil {
		return "", g.err
	}
	return g.response, nil
}

func TestNewGeminiProviderRequiresAPIKey(t *testing.T) {
	t.Parallel()

	_, err := NewGeminiProvider(config.GeminiConfig{
		Model: config.DefaultGeminiModel,
	})
	if err == nil || !strings.Contains(err.Error(), "GEMINI_API_KEY is required") {
		t.Fatalf("expected explicit api key error, got %v", err)
	}
}

func TestGeminiProviderGenerateAndStream(t *testing.T) {
	t.Parallel()

	prompts := make([]string, 0, 2)
	provider, err := NewGeminiProvider(
		config.GeminiConfig{
			APIKey: "test-api-key",
			Model:  config.DefaultGeminiModel,
		},
		WithGeminiGeneratorFactory(func(cfg config.GeminiConfig) (GeminiGenerator, error) {
			if cfg.Model != "gemini-2.0-flash-exp" {
				t.Fatalf("expected requested model to be passed to generator, got %q", cfg.Model)
			}
			if cfg.APIKey != "test-api-key" {
				t.Fatalf("expected api key to be propagated, got %q", cfg.APIKey)
			}
			return stubGeminiGenerator{
				response: "assistant reply",
				prompts:  &prompts,
			}, nil
		}),
	)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	module, err := provider.NewAssistantModule(GeminiModel("gemini-2.0-flash-exp"))
	if err != nil {
		t.Fatalf("new assistant module: %v", err)
	}

	request := GenerateRequest{
		Messages: []Message{
			{
				Role: RoleUser,
				Content: []ContentPart{
					TextContent{Text: "What is the weather?"},
				},
			},
		},
	}
	message, err := module.Generate(context.Background(), request)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if message.Role != RoleAssistant {
		t.Fatalf("expected assistant role, got %q", message.Role)
	}
	if len(message.Content) != 1 {
		t.Fatalf("expected one content part, got %d", len(message.Content))
	}
	textPart, ok := message.Content[0].(TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", message.Content[0])
	}
	if textPart.Text != "assistant reply" {
		t.Fatalf("expected assistant reply, got %q", textPart.Text)
	}
	if len(prompts) != 1 || !strings.Contains(prompts[0], "user: What is the weather?") {
		t.Fatalf("expected rendered user prompt, got %v", prompts)
	}

	stream, err := module.Stream(context.Background(), request)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	first, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("next start: %v", err)
	}
	if first.Kind() != AssistantEventKindMessageStart {
		t.Fatalf("expected message_start event, got %q", first.Kind())
	}

	second, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("next delta: %v", err)
	}
	if second.Kind() != AssistantEventKindContentDelta {
		t.Fatalf("expected content_delta event, got %q", second.Kind())
	}

	third, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("next complete: %v", err)
	}
	if third.Kind() != AssistantEventKindMessageComplete {
		t.Fatalf("expected message_complete event, got %q", third.Kind())
	}

	_, err = stream.Next(context.Background())
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestGeminiStreamReturnsErrorEventOnGenerationFailure(t *testing.T) {
	t.Parallel()

	provider, err := NewGeminiProvider(
		config.GeminiConfig{
			APIKey: "test-api-key",
			Model:  config.DefaultGeminiModel,
		},
		WithGeminiGeneratorFactory(func(config.GeminiConfig) (GeminiGenerator, error) {
			return stubGeminiGenerator{err: errors.New("boom")}, nil
		}),
	)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	module, err := provider.NewAssistantModule(DefaultGeminiModel())
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	stream, err := module.Stream(context.Background(), GenerateRequest{
		Messages: []Message{
			{
				Role: RoleUser,
				Content: []ContentPart{
					TextContent{Text: "hello"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	first, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("next start: %v", err)
	}
	if first.Kind() != AssistantEventKindMessageStart {
		t.Fatalf("expected start event, got %q", first.Kind())
	}

	second, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("next error event: %v", err)
	}
	errorEvent, ok := second.(ErrorEvent)
	if !ok {
		t.Fatalf("expected error event, got %T", second)
	}
	if errorEvent.Err == nil || !strings.Contains(errorEvent.Err.Error(), "generate assistant response: boom") {
		t.Fatalf("expected wrapped generation error, got %v", errorEvent.Err)
	}
}
