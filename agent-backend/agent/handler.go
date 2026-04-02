package agent

import (
	"encoding/json"
	"net/http"
)

func healthz() http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"status": "healthy",
			})
		},
	)
}

func agentRequest(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Prompt string `json:"prompt"`
	}
	type response struct {
		Ans string `json:"ans"`
	}
	var data request

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalide JSON", http.StatusBadRequest)
	}
}
