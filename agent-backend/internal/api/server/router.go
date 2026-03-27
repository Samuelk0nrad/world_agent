package server

import (
	"errors"

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
	var errs []error
	if r.agentHandler != nil {
		if err := r.agentHandler.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
