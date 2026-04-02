package apiservice

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"agent-backend/config"
	gemini "agent-backend/gai/ai_gemini"
	"agent-backend/gai/loop"
	"agent-backend/gai/memory"
)

func healthz(logger *log.Logger) http.HandlerFunc {
	return handler(func(w http.ResponseWriter, r *http.Request) error {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "healthy",
		})
		return nil
	}, logger)
}

func agentCall(logger *log.Logger, config *config.Env) http.HandlerFunc {
	type request struct {
		SessionId int    `json:"sessionId"`
		Prompt    string `json:"prompt"`
	}
	type response struct {
		Response memory.Message   `json:"response"`
		Messages []memory.Message `json:"messages"`
	}
	return handler(func(w http.ResponseWriter, r *http.Request) error {
		req, err := decode[request](r)
		if err != nil {
			return NewErrWithStatus(http.StatusBadRequest, err)
		}
		sessionId := req.SessionId
		provider := gemini.New(config.GeminiAPIKey)
		model, err := provider.Model(gemini.Gemini2_5Flash)
		if err != nil {
			return NewErrWithStatus(http.StatusInternalServerError, err)
		}
		var tools []loop.Tool
		tools = append(tools, loop.NewEchoTool())
		// agent, err := loop.NewAgent(model, tools, "", sessionId)
		agent, err := loop.NewAgentFromPromptFiles(
			model,
			tools,
			config.PromptPathSys,
			config.PromptPathTool,
			sessionId,
		)
		if err != nil {
			return err
		}
		agent.FollowUp(context.Background(), req.Prompt)

		messages, err := agent.MemorySystem.GetMessages(10)
		if err != nil {
			return err
		}

		encode(w, r, http.StatusOK, ApiResponse[response]{
			Data: &response{
				Response: messages[len(messages)-1],
				Messages: messages,
			},
			Message: "agent call received",
		})

		return nil
	}, logger)
}
