package gemini

import (
	"context"

	"agent-backend/pkg/gai/ai"

	"google.golang.org/genai"
)

type Model struct {
	name   string
	client *Provider
}

func (m *Model) Name() string {
	return m.name
}

func (m *Model) Generate(ctx context.Context, req ai.AIRequest) (*ai.AIResponse, error) {
	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		return nil, err
	}

	result, err := client.Models.GenerateContent(
		ctx,
		m.name,
		genai.Text(req.Promt),
		nil,
	)
	if err != nil {
		return nil, err
	}

	return &ai.AIResponse{
		Text:         result.Text(),
		InputTokens:  int(result.UsageMetadata.TotalTokenCount),
		OutputTokens: -1,
	}, nil
}
