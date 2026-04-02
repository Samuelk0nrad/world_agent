package agent

import (
	"encoding/json"
	"log"
	"net/http"
)

func healthz(logger *log.Logger) http.HandlerFunc {
	return handler(func(w http.ResponseWriter, r *http.Request) error {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "healthy",
		})
		return nil
	}, logger)
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
