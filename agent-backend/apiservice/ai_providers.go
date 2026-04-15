package apiservice

import (
	"strings"

	"agent-backend/config"
	"agent-backend/gai/ai"
	gemini "agent-backend/gai/ai_gemini"
	mistral "agent-backend/gai/ai_mistral"
)

func RegisterProviders(config config.Env) (ai.ModelRepository, error) {
	repo := ai.NewModelRepository()

	if strings.TrimSpace(config.GeminiAPIKey) != "" {
		geminiProvider := gemini.New(config.GeminiAPIKey)
		if err := repo.RegisterProvider(geminiProvider); err != nil {
			return *repo, err
		}
	}

	if strings.TrimSpace(config.MistralAPIKey) != "" {
		mistralProvider := mistral.New(config.MistralAPIKey)
		if err := repo.RegisterProvider(mistralProvider); err != nil {
			return *repo, err
		}
	}
	return *repo, nil
}
