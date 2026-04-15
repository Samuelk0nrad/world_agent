package apiservice

import (
	"agent-backend/config"
	"agent-backend/gai/ai"
	gemini "agent-backend/gai/ai_gemini"
)

func RegisterProviders(config config.Env) (ai.ModelRepository, error) {
	repo := ai.NewModelRepository()
	geminiProvider := gemini.New(config.GeminiAPIKey)
	if err := repo.RegisterProvider(geminiProvider); err != nil {
		return *repo, err
	}
	return *repo, nil
}
