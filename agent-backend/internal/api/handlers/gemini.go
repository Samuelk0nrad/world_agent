package handlers

import (
	"context"
	"net/http"

	"agent-backend/internal/config"
	"agent-backend/pkg/gai/ai"
	gemini "agent-backend/pkg/gai/ai_gemini"

	"github.com/gin-gonic/gin"
)

type GeminiHandler struct {
	GeminiModel ai.Model
}

func NewGeminiHandler(env *config.Env) *GeminiHandler {
	model := gemini.New(env.GeminiAPIKey).Model("gemini-3-flash-preview")
	return &GeminiHandler{
		GeminiModel: model,
	}
}

func (h *GeminiHandler) GetResponse(c *gin.Context) {
	req := ai.AIRequest{
		Promt:       "Hello give some random response back to the user",
		SystemPromt: "You are some wiard frient that want to make the live hard",
		MaxTokens:   100,
	}

	res, err := h.GeminiModel.Generate(context.Background(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"response": res.Text,
	})
}
