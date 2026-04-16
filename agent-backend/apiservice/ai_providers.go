package apiservice

import (
	"log"
	"strings"

	"agent-backend/config"
	"agent-backend/gai/ai"
	gemini "agent-backend/gai/ai_gemini"
	mistral "agent-backend/gai/ai_mistral"
)

func RegisterProviders(config config.Env, logger *log.Logger) (ai.ModelRepository, error) {
	repo := ai.NewModelRepository()
	ok := false

	if strings.TrimSpace(config.GeminiAPIKey) != "" {
		geminiProvider := gemini.New(config.GeminiAPIKey)
		ok = true
		if err := repo.RegisterProvider(geminiProvider); err != nil {
			return *repo, err
		}
	}

	if strings.TrimSpace(config.MistralAPIKey) != "" {
		mistralProvider := mistral.New(config.MistralAPIKey)
		ok = true
		if err := repo.RegisterProvider(mistralProvider); err != nil {
			return *repo, err
		}
	}

	if !ok {
		logger.Println("WARNING: No AI providers configured. Please set GEMINI_API_KEY or MISTRAL_API_KEY in the environment variables.")
	}

	return *repo, nil
}
