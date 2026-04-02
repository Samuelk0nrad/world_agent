package agent

import (
	"net/http"

	"agent-backend/config"
)

func NewServer(
	config *config.Env,
) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, config)
	// middleware
	return mux
}
