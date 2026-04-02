package apiservice

import (
	"log"
	"net/http"

	"agent-backend/config"
)

func addRoutes(
	mux *http.ServeMux,
	config *config.Env,
	logger *log.Logger,
) {
	mux.HandleFunc("GET /healthz", healthz(logger))
	mux.HandleFunc("POST /agent/call", agentCall(logger, config))
}
