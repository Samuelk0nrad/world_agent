package handlers

import (
	"net/http"

	"agent-backend/internal/schema"
	"agent-backend/internal/service"

	"github.com/gin-gonic/gin"
)

type AgentHandler struct {
	agentService service.AgentService
}

func NewAgentHandler(agentService service.AgentService) *AgentHandler {
	return &AgentHandler{
		agentService: agentService,
	}
}

func (h *AgentHandler) PostAgent(c *gin.Context) {
	var req schema.AgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, schema.Error{
			Code:    "invalid_request",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.agentService.ProcessAgentRequest(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, schema.Error{
			Code:    "internal_error",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AgentHandler) Close() error {
	return h.agentService.Close()
}
