package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthHandlers struct{}

func NewHealthHandlers() *HealthHandlers {
	return &HealthHandlers{}
}

func (h *HealthHandlers) GetHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
