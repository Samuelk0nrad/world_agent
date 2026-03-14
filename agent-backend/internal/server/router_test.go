package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"worldagent/agent-backend/internal/connectors"
)

type staticResponder struct {
	response string
}

func (s staticResponder) Generate(context.Context, string) (string, error) {
	return s.response, nil
}

type fakeWebSearch struct{}

func (f fakeWebSearch) ID() string {
	return connectors.WebSearchConnectorID
}

func (f fakeWebSearch) Search(context.Context, string, int) (connectors.SearchResult, error) {
	return connectors.SearchResult{Summary: "web-search summary"}, nil
}

type capturingEmailConnector struct {
	lastRequest connectors.ListMessagesRequest
}

func (f *capturingEmailConnector) ID() string {
	return connectors.GmailConnectorID
}

func (f *capturingEmailConnector) ListMessages(_ context.Context, request connectors.ListMessagesRequest) ([]connectors.EmailMessage, error) {
	f.lastRequest = request
	return []connectors.EmailMessage{{ID: "m1", Subject: "Invoice"}}, nil
}

func (f *capturingEmailConnector) GetMessage(context.Context, connectors.GetMessageRequest) (connectors.EmailMessage, error) {
	return connectors.EmailMessage{}, nil
}

func (f *capturingEmailConnector) SendMessage(context.Context, connectors.SendMessageRequest) (connectors.SendMessageResponse, error) {
	return connectors.SendMessageResponse{}, nil
}

type failingGeminiConnector struct{}

func (f failingGeminiConnector) ID() string {
	return connectors.GeminiConnectorID
}

func (f failingGeminiConnector) Generate(context.Context, string) (string, error) {
	return "", errors.New("gemini upstream unavailable")
}

func TestAgentRunEndpoint(t *testing.T) {
	t.Parallel()

	connectorRegistry := connectors.NewRegistry()
	if err := connectorRegistry.Register(fakeWebSearch{}); err != nil {
		t.Fatalf("register connector: %v", err)
	}

	cfg := Config{
		Port:              "0",
		MemoryFile:        filepath.Join(t.TempDir(), "memory.jsonl"),
		GeminiResponder:   staticResponder{response: "Gemini response"},
		ConnectorRegistry: connectorRegistry,
	}
	router := NewRouter(cfg)

	payload := map[string]any{
		"message":  "search golang concurrency patterns",
		"maxSteps": 4,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/agent/run", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestMemorySyncPullWithSince(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Port:            "0",
		MemoryFile:      filepath.Join(t.TempDir(), "memory.jsonl"),
		GeminiResponder: staticResponder{response: "ok"},
	}
	router := NewRouter(cfg)

	for _, content := range []string{"one", "two", "three"} {
		raw, err := json.Marshal(map[string]any{"source": "test", "content": content})
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/v1/memory", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d body=%s", rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/memory?since=1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Entries []struct {
			Content  string `json:"content"`
			Sequence int64  `json:"sequence"`
		} `json:"entries"`
		LatestSequence int64 `json:"latest_sequence"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(response.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(response.Entries))
	}
	if response.Entries[0].Sequence != 2 || response.Entries[1].Sequence != 3 {
		t.Fatalf("unexpected sequences %d,%d", response.Entries[0].Sequence, response.Entries[1].Sequence)
	}
	if response.LatestSequence != 3 {
		t.Fatalf("expected latest sequence 3, got %d", response.LatestSequence)
	}
}

func TestMemoryListAllStillReturnsAllEntries(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Port:            "0",
		MemoryFile:      filepath.Join(t.TempDir(), "memory.jsonl"),
		GeminiResponder: staticResponder{response: "ok"},
	}
	router := NewRouter(cfg)

	for _, content := range []string{"one", "two"} {
		raw, err := json.Marshal(map[string]any{"source": "test", "content": content})
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/v1/memory", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d body=%s", rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/memory", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var response struct {
		Entries []struct {
			Content  string `json:"content"`
			Sequence int64  `json:"sequence"`
		} `json:"entries"`
		LatestSequence int64 `json:"latest_sequence"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if len(response.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(response.Entries))
	}
	if response.Entries[0].Sequence != 1 || response.Entries[1].Sequence != 2 {
		t.Fatalf("unexpected sequences %d,%d", response.Entries[0].Sequence, response.Entries[1].Sequence)
	}
}

func TestMemorySyncPullRejectsInvalidSince(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Port:            "0",
		MemoryFile:      filepath.Join(t.TempDir(), "memory.jsonl"),
		GeminiResponder: staticResponder{response: "ok"},
	}
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/v1/memory?since=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestAgentRunReturnsExplicitErrorWhenGeminiRequestedWithoutAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GEMINI_MODEL", "")

	cfg := Config{
		Port:         "0",
		MemoryFile:   filepath.Join(t.TempDir(), "memory.jsonl"),
		LLMConnector: connectors.GeminiConnectorID,
	}
	router := NewRouter(cfg)

	payload := map[string]any{
		"message": "hello",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/agent/run", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "gemini connector requested but unavailable") {
		t.Fatalf("expected explicit gemini request error, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GEMINI_API_KEY is required") {
		t.Fatalf("expected missing API key error, got %s", rec.Body.String())
	}
}

func TestAgentRunReturnsExplicitErrorWhenGeminiConnectorFails(t *testing.T) {
	t.Parallel()

	connectorRegistry := connectors.NewRegistry()
	if err := connectorRegistry.Register(failingGeminiConnector{}); err != nil {
		t.Fatalf("register failing connector: %v", err)
	}
	if err := connectorRegistry.Register(fakeWebSearch{}); err != nil {
		t.Fatalf("register web-search connector: %v", err)
	}

	cfg := Config{
		Port:              "0",
		MemoryFile:        filepath.Join(t.TempDir(), "memory.jsonl"),
		LLMConnector:      connectors.GeminiConnectorID,
		ConnectorRegistry: connectorRegistry,
	}
	router := NewRouter(cfg)

	payload := map[string]any{
		"message": "hello",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/agent/run", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "generate response with gemini connector") {
		t.Fatalf("expected connector failure wrapper, got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "gemini upstream unavailable") {
		t.Fatalf("expected upstream error, got %s", rec.Body.String())
	}
}

func TestAgentRunEmailReturnsExplicitErrorWhenTokenMissing(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "client-secret")
	t.Setenv("GOOGLE_REDIRECT_URL", "https://example.com/callback")

	cfg := Config{
		Port:            "0",
		MemoryFile:      filepath.Join(t.TempDir(), "memory.jsonl"),
		GeminiResponder: staticResponder{response: "ok"},
	}
	router := NewRouter(cfg)

	patchRaw := []byte(`{"enabled":true}`)
	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/extensions/email", bytes.NewReader(patchRaw))
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	router.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("enable extension failed: status=%d body=%s", patchRec.Code, patchRec.Body.String())
	}

	payload := map[string]any{
		"message": "check email for updates",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/agent/run", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "gmail access token is required") {
		t.Fatalf("expected missing token error, got %s", rec.Body.String())
	}
}

func TestAgentRunEmailReturnsExplicitErrorWhenGoogleConfigMissing(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	t.Setenv("GOOGLE_REDIRECT_URL", "")

	cfg := Config{
		Port:            "0",
		MemoryFile:      filepath.Join(t.TempDir(), "memory.jsonl"),
		GeminiResponder: staticResponder{response: "ok"},
	}
	router := NewRouter(cfg)

	patchRaw := []byte(`{"enabled":true}`)
	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/extensions/email", bytes.NewReader(patchRaw))
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	router.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("enable extension failed: status=%d body=%s", patchRec.Code, patchRec.Body.String())
	}

	payload := map[string]any{
		"message":           "check email for updates",
		"googleAccessToken": "token-abc",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/agent/run", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GOOGLE_CLIENT_ID is required") {
		t.Fatalf("expected missing oauth config error, got %s", rec.Body.String())
	}
}

func TestAgentRunEmailPassesGoogleAccessTokenFromPayload(t *testing.T) {
	t.Parallel()

	emailConnector := &capturingEmailConnector{}
	connectorRegistry := connectors.NewRegistry()
	if err := connectorRegistry.Register(fakeWebSearch{}); err != nil {
		t.Fatalf("register web-search connector: %v", err)
	}
	if err := connectorRegistry.Register(emailConnector); err != nil {
		t.Fatalf("register email connector: %v", err)
	}

	cfg := Config{
		Port:              "0",
		MemoryFile:        filepath.Join(t.TempDir(), "memory.jsonl"),
		GeminiResponder:   staticResponder{response: "ok"},
		ConnectorRegistry: connectorRegistry,
	}
	router := NewRouter(cfg)

	patchRaw := []byte(`{"enabled":true}`)
	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/extensions/email", bytes.NewReader(patchRaw))
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	router.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("enable extension failed: status=%d body=%s", patchRec.Code, patchRec.Body.String())
	}

	payload := map[string]any{
		"message":           "check email for invoices",
		"googleAccessToken": "token-abc",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/agent/run", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if emailConnector.lastRequest.AccessToken != "token-abc" {
		t.Fatalf("expected token to be passed to connector, got %q", emailConnector.lastRequest.AccessToken)
	}
}
