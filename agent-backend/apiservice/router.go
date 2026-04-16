package apiservice

import (
	"log"
	"net/http"

	"agent-backend/config"
	"agent-backend/gai/ai"
	aicontext "agent-backend/gai/context"
)

func addRoutes(
	mux *http.ServeMux,
	config *config.Env,
	logger *log.Logger,
	sessionStore aicontext.SessionStore,
	modelRepo ai.ModelRepository,
) {
	agentHandler := &AgentHandler{
		logger:       logger,
		sessionStore: sessionStore,
		config:       config,
		providers:    modelRepo,
	}
	mux.HandleFunc("GET /healthz", healthz(logger))
	mux.HandleFunc("POST /agent/call", agentHandler.agentCall())
	mux.HandleFunc("GET /agent/session", agentHandler.getSessionMessages())
}
