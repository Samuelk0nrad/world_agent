package handlers

import (
	"net/http"

	"agent-backend/internal/config"
	gemini "agent-backend/pkg/gai/ai_gemini"
	"agent-backend/pkg/gai/loop"

	"github.com/gin-gonic/gin"
)

type AgentHandler struct {
	agent   *loop.Agent
	initErr error
}

func NewAgentHandler(env *config.Env) *AgentHandler {
	model, err := gemini.New(env.GeminiAPIKey).Model("gemini-3-flash-preview")
	if err != nil {
		return &AgentHandler{initErr: err}
	}

	tools := []loop.Tool{
		loop.NewEchoTool(),
	}
	agent, err := loop.NewAgentFromPromptFiles(model, tools, env.PromptPath+"/system.md", env.PromptPath+"/toolCall.md", 0)
	if err != nil {
		return &AgentHandler{initErr: err}
	}

	return &AgentHandler{agent: agent}
}

type Request struct {
	Prompt string `json:"prompt"`
}

func (h *AgentHandler) PostAgent(c *gin.Context) {
	if h.initErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": h.initErr.Error()})
		return
	}
	if h.agent == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "agent is not initialized"})
		return
	}

	var req Request
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": err.Error()})
		return
	}

	message, err := h.agent.FollowUp(c.Request.Context(), req.Prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"prompt":   req.Prompt,
		"message":  message,
		"messages": h.agent.Messages,
	})
}
