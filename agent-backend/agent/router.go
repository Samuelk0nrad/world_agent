package agent

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
	mux.HandleFunc("/healthz", healthz)
}
