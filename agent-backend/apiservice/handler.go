package apiservice

import (
	"encoding/json"
	"log"
	"net/http"

	"agent-backend/config"
	"agent-backend/gai/ai"
	"agent-backend/gai/loop"

	aicontext "agent-backend/gai/context"
)

type AgentHandler struct {
	logger       *log.Logger
	sessionStore aicontext.SessionStore
	config       *config.Env
	providers    ai.ModelRepository
}

func healthz(logger *log.Logger) http.HandlerFunc {
	return handler(func(w http.ResponseWriter, r *http.Request) error {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "healthy",
		})
		return nil
	}, logger)
}

func (h *AgentHandler) agentCall() http.HandlerFunc {
	type request struct {
		Prompt     string `json:"prompt"`
		SessionId  int    `json:"session_id"`
		NewSession bool   `json:"new_session"`
	}
	type response struct {
		Response []aicontext.Message `json:"response"`
	}
	return handler(func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()

		req, err := decode[request](r)
		if err != nil {
			return NewErrWithStatus(http.StatusBadRequest, err)
		}

		sysPrompt, err := aicontext.LoadPromptFromFile(h.config.PromptPath + "/system.md")
		if err != nil {
			return NewErrWithStatus(http.StatusInternalServerError, err)
		}

		model, err := h.providers.GetModel(h.config.Provider, h.config.Model)
		if err != nil {
			return NewErrWithStatus(http.StatusInternalServerError, err)
		}

		sessionID := req.SessionId
		if req.NewSession {
			sessionID, err = h.sessionStore.CreateSession()
			if err != nil {
				return NewErrWithStatus(http.StatusInternalServerError, err)
			}
		}
		sessionManager := aicontext.NewSessionManager(h.sessionStore, sessionID)

		agent := loop.New(
			model,
			[]loop.Tool{}, // TODO: support tools
			req.Prompt,
			sysPrompt,
			sessionManager,
			nil,
		)

		if err := agent.Loop(ctx); err != nil {
			return NewErrWithStatus(http.StatusInternalServerError, err)
		}

		message := agent.Messages()

		json.NewEncoder(w).Encode(response{
			Response: message,
		})

		return nil
	}, h.logger)
}
