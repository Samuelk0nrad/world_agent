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

	"agent-backend/internal/schema"
	"agent-backend/internal/service"
	"agent-backend/pkg/gai/ai"
	"agent-backend/pkg/gai/loop"
	"agent-backend/pkg/gai/memory"

	"github.com/gin-gonic/gin"
)

type stubModel struct{}

func (s *stubModel) Name() string { return "stub" }

func (s *stubModel) Generate(_ context.Context, _ ai.AIRequest) (*ai.AIResponse, error) {
	return &ai.AIResponse{Text: "ok"}, nil
}

func (s *stubModel) Close() error { return nil }

// mockAgentService for testing that returns errors or success based on setup
type mockAgentService struct {
	shouldError bool
	errorMsg    string
}

func (m *mockAgentService) ProcessAgentRequest(ctx context.Context, req *schema.AgentRequest) (*schema.AgentResponse, error) {
	if m.shouldError {
		return nil, &testError{msg: m.errorMsg}
	}
	return &schema.AgentResponse{
		Prompt:   req.Prompt,
		Message:  "ok",
		Messages: []memory.Message{},
	}, nil
}

func (m *mockAgentService) Close() error { return nil }

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestPostAgentRequiresValidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewAgentHandler(&mockAgentService{shouldError: false})
	router := gin.New()
	router.POST("/api/agent", handler.PostAgent)

	body, _ := json.Marshal(map[string]any{
		"prompt": "hello",
		// missing sessionId
	})
	req := httptest.NewRequest(http.MethodPost, "/api/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing session id, got %d", res.Code)
	}
}

func TestPostAgentHandlesServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewAgentHandler(&mockAgentService{
		shouldError: true,
		errorMsg:    "service error",
	})
	router := gin.New()
	router.POST("/api/agent", handler.PostAgent)

	body, _ := json.Marshal(map[string]any{
		"prompt":    "hello",
		"sessionId": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for service error, got %d", res.Code)
	}
}

func TestPostAgentSuccessResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewAgentHandler(&mockAgentService{shouldError: false})
	router := gin.New()
	router.POST("/api/agent", handler.PostAgent)

	body, _ := json.Marshal(map[string]any{
		"prompt":    "hello",
		"sessionId": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/agent", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	var response schema.AgentResponse
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Prompt != "hello" {
		t.Errorf("expected prompt 'hello', got '%s'", response.Prompt)
	}
	if response.Message != "ok" {
		t.Errorf("expected message 'ok', got '%s'", response.Message)
	}
}

// Integration-style test that uses the real service layer
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

	svc := &testAgentService{
		model:      &stubModel{},
		tools:      []loop.Tool{},
		promptPath: dir,
	}

	handler := NewAgentHandler(svc)
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

	// Access the service's sessions through the test service
	s1 := svc.sessions[1]
	s2 := svc.sessions[2]
	if s1 == nil || s2 == nil {
		t.Fatalf("expected both session agents to exist")
	}

	msgs1, err := s1.Agent.MemorySystem.GetMessages(0)
	if err != nil {
		t.Fatalf("session 1 GetMessages error: %v", err)
	}
	msgs2, err := s2.Agent.MemorySystem.GetMessages(0)
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

// testAgentService is a test implementation that exposes internal state
type testAgentService struct {
	model      ai.Model
	tools      []loop.Tool
	promptPath string
	sessions   map[int]*service.SessionAgent
}

func (s *testAgentService) ProcessAgentRequest(ctx context.Context, req *schema.AgentRequest) (*schema.AgentResponse, error) {
	if req.SessionID <= 0 {
		return nil, &testError{msg: "sessionId must be a positive integer"}
	}

	sessionAgent, err := s.getOrCreateSessionAgent(req.SessionID)
	if err != nil {
		return nil, err
	}

	sessionAgent.Mu.Lock()
	defer sessionAgent.Mu.Unlock()

	message, err := sessionAgent.Agent.FollowUp(ctx, req.Prompt)
	if err != nil {
		return nil, err
	}

	messages, err := sessionAgent.Agent.MemorySystem.GetMessages(0)
	if err != nil {
		return nil, err
	}

	return &schema.AgentResponse{
		Prompt:   req.Prompt,
		Message:  message,
		Messages: messages,
	}, nil
}

func (s *testAgentService) getOrCreateSessionAgent(sessionID int) (*service.SessionAgent, error) {
	if s.sessions == nil {
		s.sessions = make(map[int]*service.SessionAgent)
	}

	if existing, ok := s.sessions[sessionID]; ok {
		return existing, nil
	}

	agent, err := loop.NewAgentFromPromptFiles(
		s.model,
		s.tools,
		s.promptPath+"/system.md",
		s.promptPath+"/toolCall.md",
		sessionID,
	)
	if err != nil {
		return nil, err
	}

	created := &service.SessionAgent{Agent: agent}
	s.sessions[sessionID] = created
	return created, nil
}

func (s *testAgentService) Close() error {
	return nil
}
