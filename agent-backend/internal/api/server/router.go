package server

import (
	"agent-backend/internal/api/handlers"
	"agent-backend/internal/config"

	"github.com/gin-gonic/gin"
)

func NewRouter(env *config.Env) *gin.Engine {
	router := gin.Default()

	healthHandler := handlers.NewHealthHandlers()
	geminiHandler := handlers.NewGeminiHandler(env)

	api := router.Group("/api")
	{
		api.GET("/health", healthHandler.GetHealth)
		api.GET("/ai", geminiHandler.GetResponse)
	}

	return router
}
