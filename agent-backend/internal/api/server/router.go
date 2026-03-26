package server

import (
	"errors"

	"agent-backend/internal/api/handlers"
	"agent-backend/internal/config"

	"github.com/gin-gonic/gin"
)

type RouterRuntime struct {
	Router        *gin.Engine
	geminiHandler *handlers.GeminiHandler
	agentHandler  *handlers.AgentHandler
}

func NewRouter(env *config.Env) *RouterRuntime {
	router := gin.Default()

	healthHandler := handlers.NewHealthHandlers()
	geminiHandler := handlers.NewGeminiHandler(env)
	agentHandler := handlers.NewAgentHandler(env)

	api := router.Group("/api")
	{
		api.GET("/health", healthHandler.GetHealth)
		api.GET("/ai", geminiHandler.GetResponse)
		api.POST("/agent", agentHandler.PostAgent)
	}

	return &RouterRuntime{
		Router:        router,
		geminiHandler: geminiHandler,
		agentHandler:  agentHandler,
	}
}

func (r *RouterRuntime) Close() error {
	if r == nil {
		return nil
	}
	var errs []error
	if r.agentHandler != nil {
		if err := r.agentHandler.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.geminiHandler != nil {
		if err := r.geminiHandler.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
