package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrResponderUnavailable = errors.New("gemini responder is not configured")

type Responder interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

func GenerateAgentResponse(ctx context.Context, responder Responder, prompt string) (string, error) {
	if responder == nil {
		return "", ErrResponderUnavailable
	}

	trimmedPrompt := strings.TrimSpace(prompt)
	if trimmedPrompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	response, err := responder.Generate(ctx, trimmedPrompt)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(response) == "" {
		return "", fmt.Errorf("gemini returned an empty response")
	}

	return response, nil
}
