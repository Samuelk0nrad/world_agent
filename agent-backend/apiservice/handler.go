package apiservice

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"agent-backend/config"
	gemini "agent-backend/gai/ai_gemini"
	"agent-backend/gai/loop"
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
		Prompt string `json:"prompt"`
	}
	type response struct {
		Response []loop.Iteration `json:"response"`
	}
	return handler(func(w http.ResponseWriter, r *http.Request) error {
		req, err := decode[request](r)
		if err != nil {
			return NewErrWithStatus(http.StatusBadRequest, err)
		}

		provider := gemini.New(config.GeminiAPIKey)
		model, err := provider.Model(gemini.Gemini2_5FlashLite)
		if err != nil {
			return NewErrWithStatus(http.StatusInternalServerError, err)
		}

		var tools []loop.Tool
		tools = append(tools, loop.NewEchoTool())

		agent := loop.New(
			model,
			tools,
			req.Prompt,
		)
		if err != nil {
			return err
		}

		if err = agent.Loop(
			context.Background(),
			req.Prompt,
			func(iterations []loop.Iteration) string {
				var builder strings.Builder
				loop.BuildIterationsString(&builder, iterations)
				return builder.String()
			},
			func(req loop.ToolRequest, res *loop.ToolResponse) error {
				return nil
			},
		); err != nil {
			return err
		}

		if err = encode(w, r, http.StatusOK, ApiResponse[response]{
			Data: &response{
				Response: agent.Iterations,
			},
			Message: "success",
		}); err != nil {
			return nil
		}

		return nil
	}, logger)
}
