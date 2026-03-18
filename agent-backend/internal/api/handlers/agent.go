package handlers

import (
	"context"
	"net/http"

	"agent-backend/internal/config"
	gemini "agent-backend/pkg/gai/ai_gemini"
	"agent-backend/pkg/gai/loop"

	"github.com/gin-gonic/gin"
)

type AgentHandler struct {
	Agent loop.Agent
}

func NewAgentHandler(env *config.Env) *AgentHandler {
	model := gemini.New(env.GeminiAPIKey).Model("gemini-3-flash-preview")
	var tools []loop.Tool
	agent := loop.NewAgent(model, tools, "you are a the World Agent")
	return &AgentHandler{
		*agent,
	}
}

type Request struct {
	Prompt string `json:"prompt"`
}

func (h *AgentHandler) PostAgentAgent(c *gin.Context) {
	var req Request
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"err": err.Error(),
		})
		return
	}

	message, err := h.Agent.FollowUp(context.Background(), req.Prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"err": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"prompt":  req.Prompt,
		"message": message.Text,
	})
}
