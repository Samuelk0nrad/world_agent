// Package ai defines provider-neutral assistant contracts.
//
// The backend is currently Gemini-first (see Gemini model helpers and provider),
// while keeping provider/module registration extension points so additional model
// providers can be integrated without changing runtime API contracts.
package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	GeminiProviderID     = "gemini"
	GeminiDefaultModelID = "gemini-2.0-flash"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

func (r Role) Validate() error {
	switch r {
	case RoleSystem, RoleUser, RoleAssistant, RoleTool:
		return nil
	case "":
		return fmt.Errorf("message role is required")
	default:
		return fmt.Errorf("unsupported message role %q", r)
	}
}

type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeToolCall   ContentType = "tool_call"
	ContentTypeToolResult ContentType = "tool_result"
)

func (t ContentType) Validate() error {
	switch t {
	case ContentTypeText, ContentTypeToolCall, ContentTypeToolResult:
		return nil
	case "":
		return fmt.Errorf("content type is required")
	default:
		return fmt.Errorf("unsupported content type %q", t)
	}
}

type ContentPart interface {
	Type() ContentType
}

type TextContent struct {
	Text string
}

func (TextContent) Type() ContentType {
	return ContentTypeText
}

func (c TextContent) Validate() error {
	if strings.TrimSpace(c.Text) == "" {
		return fmt.Errorf("text content is required")
	}
	return nil
}

type ToolCallContent struct {
	Call ToolCall
}

func (ToolCallContent) Type() ContentType {
	return ContentTypeToolCall
}

func (c ToolCallContent) Validate() error {
	return c.Call.Validate()
}

type ToolResultContent struct {
	Result ToolResult
}

func (ToolResultContent) Type() ContentType {
	return ContentTypeToolResult
}

func (c ToolResultContent) Validate() error {
	return c.Result.Validate()
}

type Message struct {
	Role       Role
	Content    []ContentPart
	Name       string
	ToolCallID string
}

func (m Message) Validate() error {
	var validationErrors []error
	if err := m.Role.Validate(); err != nil {
		validationErrors = append(validationErrors, err)
	}
	if len(m.Content) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("message content is required"))
	}
	for idx, part := range m.Content {
		if err := validateContentPart(part); err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("content[%d]: %w", idx, err))
		}
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return errors.Join(validationErrors...)
}

type ToolCall struct {
	ID    string
	Name  string
	Input any
}

func (c ToolCall) Validate() error {
	var validationErrors []error
	if strings.TrimSpace(c.ID) == "" {
		validationErrors = append(validationErrors, fmt.Errorf("tool call id is required"))
	}
	if strings.TrimSpace(c.Name) == "" {
		validationErrors = append(validationErrors, fmt.Errorf("tool call name is required"))
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return errors.Join(validationErrors...)
}

type ToolResult struct {
	ToolCallID string
	Name       string
	Output     any
	IsError    bool
}

func (r ToolResult) Validate() error {
	var validationErrors []error
	if strings.TrimSpace(r.ToolCallID) == "" {
		validationErrors = append(validationErrors, fmt.Errorf("tool result toolCallID is required"))
	}
	if strings.TrimSpace(r.Name) == "" {
		validationErrors = append(validationErrors, fmt.Errorf("tool result name is required"))
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return errors.Join(validationErrors...)
}

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema map[string]any
}

func (d ToolDefinition) Validate() error {
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("tool name is required")
	}
	return nil
}

type GenerationConfig struct {
	Temperature   float64
	MaxOutputSize int
}

func (c GenerationConfig) Validate() error {
	if c.Temperature < 0 {
		return fmt.Errorf("temperature must be greater than or equal to 0")
	}
	if c.MaxOutputSize < 0 {
		return fmt.Errorf("max output size must be greater than or equal to 0")
	}
	return nil
}

type GenerateRequest struct {
	Messages []Message
	Tools    []ToolDefinition
	Config   GenerationConfig
}

func (r GenerateRequest) Validate() error {
	var validationErrors []error
	if len(r.Messages) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("at least one message is required"))
	}
	for idx, message := range r.Messages {
		if err := message.Validate(); err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("messages[%d]: %w", idx, err))
		}
	}
	for idx, tool := range r.Tools {
		if err := tool.Validate(); err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("tools[%d]: %w", idx, err))
		}
	}
	if err := r.Config.Validate(); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("config: %w", err))
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return errors.Join(validationErrors...)
}

type ModelCapabilities struct {
	SupportsTools     bool
	SupportsStreaming bool
}

type ModelDescriptor interface {
	ProviderID() string
	ModelID() string
	DisplayName() string
	Capabilities() ModelCapabilities
}

type StaticModelDescriptor struct {
	Provider string
	ID       string
	Name     string
	Features ModelCapabilities
}

func (m StaticModelDescriptor) ProviderID() string {
	return normalizeProviderID(m.Provider)
}

func (m StaticModelDescriptor) ModelID() string {
	return strings.TrimSpace(m.ID)
}

func (m StaticModelDescriptor) DisplayName() string {
	name := strings.TrimSpace(m.Name)
	if name != "" {
		return name
	}
	return m.ModelID()
}

func (m StaticModelDescriptor) Capabilities() ModelCapabilities {
	return m.Features
}

func ValidateModelDescriptor(model ModelDescriptor) error {
	if model == nil {
		return fmt.Errorf("model is required")
	}
	if normalizeProviderID(model.ProviderID()) == "" {
		return fmt.Errorf("model provider id is required")
	}
	if strings.TrimSpace(model.ModelID()) == "" {
		return fmt.Errorf("model id is required")
	}
	return nil
}

func GeminiModel(modelID string) StaticModelDescriptor {
	trimmedModelID := strings.TrimSpace(modelID)
	if trimmedModelID == "" {
		trimmedModelID = GeminiDefaultModelID
	}
	return StaticModelDescriptor{
		Provider: GeminiProviderID,
		ID:       trimmedModelID,
		Name:     trimmedModelID,
		Features: ModelCapabilities{
			SupportsTools:     true,
			SupportsStreaming: true,
		},
	}
}

func DefaultGeminiModel() StaticModelDescriptor {
	return StaticModelDescriptor{
		Provider: GeminiProviderID,
		ID:       GeminiDefaultModelID,
		Name:     "Gemini 2.0 Flash",
		Features: ModelCapabilities{
			SupportsTools:     true,
			SupportsStreaming: true,
		},
	}
}

type AssistantModule interface {
	Model() ModelDescriptor
	Generate(ctx context.Context, request GenerateRequest) (Message, error)
	Stream(ctx context.Context, request GenerateRequest) (AssistantEventStream, error)
}

type AssistantModuleFactory func(model ModelDescriptor) (AssistantModule, error)

type Provider interface {
	ID() string
	NewAssistantModule(model ModelDescriptor) (AssistantModule, error)
}

type StaticProvider struct {
	ProviderID string
	Factory    AssistantModuleFactory
}

func (p StaticProvider) ID() string {
	return strings.TrimSpace(p.ProviderID)
}

func (p StaticProvider) NewAssistantModule(model ModelDescriptor) (AssistantModule, error) {
	if p.Factory == nil {
		return nil, fmt.Errorf("provider module factory is required")
	}
	if err := ValidateModelDescriptor(model); err != nil {
		return nil, err
	}
	return p.Factory(model)
}

func NewStaticProvider(providerID string, factory AssistantModuleFactory) Provider {
	return StaticProvider{
		ProviderID: providerID,
		Factory:    factory,
	}
}

func validateContentPart(part ContentPart) error {
	if part == nil {
		return fmt.Errorf("content part is nil")
	}
	contentType := part.Type()
	if err := contentType.Validate(); err != nil {
		return err
	}

	switch typed := part.(type) {
	case TextContent:
		return typed.Validate()
	case *TextContent:
		if typed == nil {
			return fmt.Errorf("text content is nil")
		}
		return typed.Validate()
	case ToolCallContent:
		return typed.Validate()
	case *ToolCallContent:
		if typed == nil {
			return fmt.Errorf("tool call content is nil")
		}
		return typed.Validate()
	case ToolResultContent:
		return typed.Validate()
	case *ToolResultContent:
		if typed == nil {
			return fmt.Errorf("tool result content is nil")
		}
		return typed.Validate()
	default:
		return nil
	}
}

var _ ContentPart = TextContent{}
var _ ContentPart = ToolCallContent{}
var _ ContentPart = ToolResultContent{}
var _ ModelDescriptor = StaticModelDescriptor{}
var _ Provider = StaticProvider{}
