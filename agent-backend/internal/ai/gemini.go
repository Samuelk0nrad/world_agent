package ai

import (
	"context"
	"fmt"
	"strings"

	"worldagent/agent-backend/internal/config"
	"worldagent/agent-backend/internal/llm"
)

const (
	geminiFinishReasonStop = "stop"
)

type GeminiGenerator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type GeminiGeneratorFactory func(cfg config.GeminiConfig) (GeminiGenerator, error)

type GeminiProviderOption func(*GeminiProvider)

type GeminiProvider struct {
	baseConfig       config.GeminiConfig
	generatorFactory GeminiGeneratorFactory
}

func NewGeminiProviderFromEnv(options ...GeminiProviderOption) (*GeminiProvider, error) {
	return NewGeminiProvider(config.LoadGeminiConfig(), options...)
}

func NewGeminiProvider(cfg config.GeminiConfig, options ...GeminiProviderOption) (*GeminiProvider, error) {
	provider := &GeminiProvider{
		baseConfig: cfg,
		generatorFactory: func(cfg config.GeminiConfig) (GeminiGenerator, error) {
			return llm.NewGeminiClient(cfg)
		},
	}
	for _, option := range options {
		if option != nil {
			option(provider)
		}
	}
	if provider.generatorFactory == nil {
		return nil, fmt.Errorf("gemini generator factory is required")
	}
	if strings.TrimSpace(provider.baseConfig.APIKey) == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required")
	}
	return provider, nil
}

func WithGeminiGeneratorFactory(factory GeminiGeneratorFactory) GeminiProviderOption {
	return func(provider *GeminiProvider) {
		if factory != nil {
			provider.generatorFactory = factory
		}
	}
}

func (p *GeminiProvider) ID() string {
	return GeminiProviderID
}

func (p *GeminiProvider) NewAssistantModule(model ModelDescriptor) (AssistantModule, error) {
	if err := ValidateModelDescriptor(model); err != nil {
		return nil, err
	}
	modelProviderID := normalizeProviderID(model.ProviderID())
	if modelProviderID != GeminiProviderID {
		return nil, fmt.Errorf("model provider %q is not supported by Gemini provider", modelProviderID)
	}

	moduleModel := GeminiModel(model.ModelID())
	moduleModel.Name = model.DisplayName()
	moduleModel.Features = model.Capabilities()

	clientConfig := p.baseConfig
	clientConfig.Model = moduleModel.ModelID()
	generator, err := p.generatorFactory(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("create gemini generator: %w", err)
	}

	return &GeminiAssistantModule{
		model:     moduleModel,
		generator: generator,
	}, nil
}

type GeminiAssistantModule struct {
	model     ModelDescriptor
	generator GeminiGenerator
}

func (m *GeminiAssistantModule) Model() ModelDescriptor {
	return m.model
}

func (m *GeminiAssistantModule) Generate(ctx context.Context, request GenerateRequest) (Message, error) {
	if m == nil {
		return Message{}, fmt.Errorf("assistant module is nil")
	}
	if m.generator == nil {
		return Message{}, fmt.Errorf("gemini generator is required")
	}
	if err := request.Validate(); err != nil {
		return Message{}, err
	}

	prompt := renderMessagesAsPrompt(request.Messages)
	response, err := m.generator.Generate(ctx, prompt)
	if err != nil {
		return Message{}, fmt.Errorf("generate assistant response: %w", err)
	}

	return Message{
		Role: RoleAssistant,
		Content: []ContentPart{
			TextContent{Text: strings.TrimSpace(response)},
		},
	}, nil
}

func (m *GeminiAssistantModule) Stream(ctx context.Context, request GenerateRequest) (AssistantEventStream, error) {
	if m == nil {
		return nil, fmt.Errorf("assistant module is nil")
	}
	message, err := m.Generate(ctx, request)
	if err != nil {
		return NewSliceEventStream(
			MessageStartEvent{Model: m.model},
			ErrorEvent{Err: err},
		), nil
	}

	events := []AssistantEvent{
		MessageStartEvent{Model: m.model},
	}
	for _, part := range message.Content {
		events = append(events, ContentDeltaEvent{Delta: part})
	}
	events = append(events, MessageCompleteEvent{
		Message:      message,
		FinishReason: geminiFinishReasonStop,
	})
	return NewSliceEventStream(events...), nil
}

func RegisterGeminiProviderFromEnv(options ...GeminiProviderOption) error {
	// Gemini is the default production provider; additional providers can be
	// registered through the same registry APIs.
	provider, err := NewGeminiProviderFromEnv(options...)
	if err != nil {
		return err
	}
	return RegisterProvider(provider)
}

func renderMessagesAsPrompt(messages []Message) string {
	var builder strings.Builder
	for _, message := range messages {
		builder.WriteString(string(message.Role))
		builder.WriteString(": ")
		builder.WriteString(renderContentParts(message.Content))
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func renderContentParts(parts []ContentPart) string {
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		switch typed := part.(type) {
		case TextContent:
			segments = append(segments, strings.TrimSpace(typed.Text))
		case *TextContent:
			if typed != nil {
				segments = append(segments, strings.TrimSpace(typed.Text))
			}
		case ToolCallContent:
			segments = append(segments, fmt.Sprintf("tool_call[%s]: %s", typed.Call.ID, typed.Call.Name))
		case *ToolCallContent:
			if typed != nil {
				segments = append(segments, fmt.Sprintf("tool_call[%s]: %s", typed.Call.ID, typed.Call.Name))
			}
		case ToolResultContent:
			segments = append(segments, fmt.Sprintf("tool_result[%s]: %s", typed.Result.ToolCallID, typed.Result.Name))
		case *ToolResultContent:
			if typed != nil {
				segments = append(segments, fmt.Sprintf("tool_result[%s]: %s", typed.Result.ToolCallID, typed.Result.Name))
			}
		default:
			segments = append(segments, fmt.Sprintf("%v", part))
		}
	}
	return strings.TrimSpace(strings.Join(segments, "\n"))
}

var _ Provider = (*GeminiProvider)(nil)
var _ AssistantModule = (*GeminiAssistantModule)(nil)
