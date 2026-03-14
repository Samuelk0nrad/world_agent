package connectors

import (
	"context"
	"fmt"
)

const GeminiConnectorID = "gemini"

type TextGenerationConnector interface {
	Connector
	Generate(ctx context.Context, prompt string) (string, error)
}

func GetTextGenerationConnector(registry *Registry, id string) (TextGenerationConnector, error) {
	if registry == nil {
		return nil, fmt.Errorf("connector registry is not configured")
	}

	connectorID := normalizeConnectorID(id)
	if connectorID == "" {
		return nil, fmt.Errorf("text generation connector id is required")
	}

	connector, ok := registry.Get(connectorID)
	if !ok {
		return nil, fmt.Errorf("connector %q is not registered", connectorID)
	}

	textGenerationConnector, ok := connector.(TextGenerationConnector)
	if !ok {
		return nil, fmt.Errorf("connector %q does not implement text generation", connectorID)
	}

	return textGenerationConnector, nil
}
