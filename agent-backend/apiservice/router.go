package apiservice

import (
	"log"
	"net/http"

	"agent-backend/config"
	aicontext "agent-backend/gai/context"
)

func addRoutes(
	mux *http.ServeMux,
	config *config.Env,
	logger *log.Logger,
	sessionStore aicontext.SessionStore,
) {
	mux.HandleFunc("GET /healthz", healthz(logger))
	mux.HandleFunc("POST /agent/call", agentCall(logger, sessionStore, config))
}
