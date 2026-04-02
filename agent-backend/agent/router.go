package agent

import (
	"net/http"

	"agent-backend/config"
)

func addRoutes(
	mux *http.ServeMux,
	config *config.Env,
) {
	mux.HandleFunc("/healthz", healthz)
}
