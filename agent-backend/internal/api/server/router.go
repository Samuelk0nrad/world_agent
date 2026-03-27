package server

import (
	"errors"
	"log"

	"agent-backend/internal/api/handlers"
	"agent-backend/internal/config"
	"agent-backend/internal/service"

	"github.com/gin-gonic/gin"
)

type RouterRuntime struct {
	Router       *gin.Engine
	agentHandler *handlers.AgentHandler
}

func NewRouter(env *config.Env) *RouterRuntime {
	router := gin.Default()

	healthHandler := handlers.NewHealthHandlers()

	agentService, err := service.NewAgentService(env)
	if err != nil {
		log.Fatalf("Failed to initialize agent service: %v", err)
	}
	agentHandler := handlers.NewAgentHandler(agentService)

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
