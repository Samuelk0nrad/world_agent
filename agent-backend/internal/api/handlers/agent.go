package handlers

import (
	"net/http"
	"sync"

	"agent-backend/internal/config"
	"agent-backend/pkg/gai/ai"
	gemini "agent-backend/pkg/gai/ai_gemini"
	"agent-backend/pkg/gai/loop"

	"github.com/gin-gonic/gin"
)

type AgentHandler struct {
	model      ai.Model
	tools      []loop.Tool
	promptPath string
	sessions   map[int]*sessionAgent
	sessionsMu sync.Mutex
	initErr    error
}

type sessionAgent struct {
	agent *loop.Agent
	mu    sync.Mutex
}

func NewAgentHandler(env *config.Env) *AgentHandler {
	model, err := gemini.New(env.GeminiAPIKey).Model(gemini.Gemini2_5Flash)
	if err != nil {
		return &AgentHandler{initErr: err}
	}

	return &AgentHandler{
		model:      model,
		tools:      []loop.Tool{loop.NewEchoTool()},
		promptPath: env.PromptPath,
		sessions:   make(map[int]*sessionAgent),
	}
}

type Request struct {
	Prompt    string `json:"prompt"`
	SessionID int    `json:"sessionId"`
}

func (h *AgentHandler) PostAgent(c *gin.Context) {
	if h.initErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": h.initErr.Error()})
		return
	}
	if h.model == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "agent model is not initialized"})
		return
	}

	var req Request
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": err.Error()})
		return
	}
	if req.SessionID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"err": "sessionId must be a positive integer"})
		return
	}

	sessionAgent, err := h.getOrCreateSessionAgent(req.SessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": err.Error()})
		return
	}

	sessionAgent.mu.Lock()
	defer sessionAgent.mu.Unlock()

	message, err := sessionAgent.agent.FollowUp(c.Request.Context(), req.Prompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": err.Error()})
		return
	}

	messages, err := sessionAgent.agent.MemorySystem.GetMessages(0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"prompt":   req.Prompt,
		"message":  message,
		"messages": messages,
	})
}

func (h *AgentHandler) getOrCreateSessionAgent(sessionID int) (*sessionAgent, error) {
	h.sessionsMu.Lock()
	defer h.sessionsMu.Unlock()

	if existing, ok := h.sessions[sessionID]; ok {
		return existing, nil
	}

	agent, err := loop.NewAgentFromPromptFiles(
		h.model,
		h.tools,
		h.promptPath+"/system.md",
		h.promptPath+"/toolCall.md",
		sessionID,
	)
	if err != nil {
		return nil, err
	}

	created := &sessionAgent{agent: agent}
	h.sessions[sessionID] = created
	return created, nil
}

func (h *AgentHandler) Close() error {
	h.sessionsMu.Lock()
	defer h.sessionsMu.Unlock()

	if h.model != nil {
		if err := h.model.Close(); err != nil {
			return err
		}
	}
	h.sessions = map[int]*sessionAgent{}
	return nil
}
