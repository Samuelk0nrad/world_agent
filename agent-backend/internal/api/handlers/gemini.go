package handlers

import (
	"net/http"

	"agent-backend/internal/config"
	"agent-backend/pkg/gai/ai"
	gemini "agent-backend/pkg/gai/ai_gemini"

	"github.com/gin-gonic/gin"
)

type GeminiHandler struct {
	geminiModel ai.Model
	initErr     error
}

func NewGeminiHandler(env *config.Env) *GeminiHandler {
	model, err := gemini.New(env.GeminiAPIKey).Model("gemini-3-flash-preview")
	if err != nil {
		return &GeminiHandler{initErr: err}
	}

	return &GeminiHandler{geminiModel: model}
}

func (h *GeminiHandler) GetResponse(c *gin.Context) {
	if h.initErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": h.initErr.Error()})
		return
	}
	if h.geminiModel == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gemini model is not initialized"})
		return
	}

	req := ai.AIRequest{
		Prompt:       "Hello give some random response back to the user",
		SystemPrompt: "You are some weird friend that want to make the live hard",
		MaxTokens:    100,
	}

	res, err := h.geminiModel.Generate(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":         err.Error(),
			"error_message": "some internal error occurred",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"response": res.Text})
}

func (h *GeminiHandler) Close() error {
	if h.geminiModel == nil {
		return nil
	}
	return h.geminiModel.Close()
}
