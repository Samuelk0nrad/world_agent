package server

import (
	"agent-backend/internal/api/handlers"
	"agent-backend/internal/config"

	"github.com/gin-gonic/gin"
)

type RouterRuntime struct {
	Router       *gin.Engine
	agentHandler *handlers.AgentHandler
}

func NewRouter(env *config.Env) *RouterRuntime {
	router := gin.Default()

	healthHandler := handlers.NewHealthHandlers()
	agentHandler := handlers.NewAgentHandler(env)

	api := router.Group("/api")
	{
		api.GET("/health", healthHandler.GetHealth)
		api.POST("/agent", agentHandler.PostAgent)
	}

	return &RouterRuntime{
		Router:       router,
		agentHandler: agentHandler,
	}
}

func (r *RouterRuntime) Close() error {
	if r == nil {
		return nil
	}
	if r.agentHandler != nil {
		if err := r.agentHandler.Close(); err != nil {
			return err
		}
	}
	return nil
}
