package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"agent-backend/pkg/gai/ai"

	"github.com/gin-gonic/gin"
)

type stubModel struct{}

func (s *stubModel) Name() string { return "stub" }

func (s *stubModel) Generate(_ context.Context, _ ai.AIRequest) (*ai.AIResponse, error) {
	return &ai.AIResponse{Text: "ok"}, nil
}

func (s *stubModel) Close() error { return nil }

func TestPostAgentRequiresInitializedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AgentHandler{initErr: nil}
	router := gin.New()
	router.POST("/api/agent", handler.PostAgent)

	body, _ := json.Marshal(map[string]any{
		"prompt":    "hello",
		"sessionId": 0,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when model not initialized, got %d", res.Code)
	}
}

func TestPostAgentRequiresPositiveSessionID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AgentHandler{
		model:      &stubModel{},
		tools:      nil,
		promptPath: "",
		sessions:   map[int]*sessionAgent{},
	}
	router := gin.New()
	router.POST("/api/agent", handler.PostAgent)

	body, _ := json.Marshal(map[string]any{
		"prompt":    "hello",
		"sessionId": -1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid session id, got %d", res.Code)
	}
}

func TestPostAgentRequiresSessionIDField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &AgentHandler{
		model:    &stubModel{},
		tools:    nil,
		sessions: map[int]*sessionAgent{},
	}
	router := gin.New()
	router.POST("/api/agent", handler.PostAgent)

	body, _ := json.Marshal(map[string]any{
		"prompt": "hello",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing session id, got %d", res.Code)
	}
}

func TestPostAgentSessionIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	writePrompt := func(name, value string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatalf("failed to write prompt file %s: %v", name, err)
		}
	}
	writePrompt("system.md", "system")
	writePrompt("toolCall.md", "tool")

	handler := &AgentHandler{
		model:      &stubModel{},
		tools:      nil,
		promptPath: dir,
		sessions:   map[int]*sessionAgent{},
	}
	router := gin.New()
	router.POST("/api/agent", handler.PostAgent)

	post := func(sessionID int) int {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"prompt":    "hello",
			"sessionId": sessionID,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/agent", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res.Code
	}

	if code := post(1); code != http.StatusOK {
		t.Fatalf("expected 200 for session 1, got %d", code)
	}
	if code := post(2); code != http.StatusOK {
		t.Fatalf("expected 200 for session 2, got %d", code)
	}
	if code := post(1); code != http.StatusOK {
		t.Fatalf("expected 200 for session 1 second request, got %d", code)
	}

	s1 := handler.sessions[1]
	s2 := handler.sessions[2]
	if s1 == nil || s2 == nil {
		t.Fatalf("expected both session agents to exist")
	}

	msgs1, err := s1.agent.MemorySystem.GetMessages(0)
	if err != nil {
		t.Fatalf("session 1 GetMessages error: %v", err)
	}
	msgs2, err := s2.agent.MemorySystem.GetMessages(0)
	if err != nil {
		t.Fatalf("session 2 GetMessages error: %v", err)
	}

	if len(msgs1) != 4 {
		t.Fatalf("expected 4 messages for session 1, got %d", len(msgs1))
	}
	if len(msgs2) != 2 {
		t.Fatalf("expected 2 messages for session 2, got %d", len(msgs2))
	}
}
