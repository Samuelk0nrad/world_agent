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
	"worldagent/agent-backend/internal/observability"

	"github.com/spf13/viper"
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

func TestAgentRunEndpointPreservesResultContract(t *testing.T) {
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

	raw := []byte(`{"message":"search golang concurrency patterns","maxSteps":4}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/run", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Result struct {
			Reply string `json:"reply"`
			Steps []struct {
				Name   string `json:"name"`
				Detail string `json:"detail"`
			} `json:"steps"`
		} `json:"result"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response.Result.Reply != "Gemini response" {
		t.Fatalf("expected result.reply to be preserved, got %q", response.Result.Reply)
	}
	if len(response.Result.Steps) == 0 {
		t.Fatal("expected non-empty result.steps")
	}
	if response.Result.Steps[0].Name != "ingest" {
		t.Fatalf("expected first result step to remain ingest, got %q", response.Result.Steps[0].Name)
	}
}

func TestAgentRunEndpointResultOmitsInternalLoopFields(t *testing.T) {
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

	raw := []byte(`{"message":"search golang concurrency patterns","maxSteps":4}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/agent/run", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %#v", response["result"])
	}
	for _, key := range []string{"state", "events", "completed", "finalMessage"} {
		if _, exists := result[key]; exists {
			t.Fatalf("did not expect internal loop field %q in /v1/agent/run result", key)
		}
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

func TestLogsEndpointReturnsEventsForUI(t *testing.T) {
	t.Parallel()

	connectorRegistry := connectors.NewRegistry()
	if err := connectorRegistry.Register(fakeWebSearch{}); err != nil {
		t.Fatalf("register connector: %v", err)
	}

	cfg := Config{
		Port:              "0",
		MemoryFile:        filepath.Join(t.TempDir(), "memory.jsonl"),
		GeminiResponder:   staticResponder{response: "ok"},
		ConnectorRegistry: connectorRegistry,
		LogAPIEnabled:     true,
		LogEventsEnabled:  true,
	}
	router := NewRouter(cfg)

	raw := []byte(`{"message":"hello","maxSteps":2}`)
	runReq := httptest.NewRequest(http.MethodPost, "/v1/agent/run", bytes.NewReader(raw))
	runReq.Header.Set("Content-Type", "application/json")
	runRec := httptest.NewRecorder()
	router.ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", runRec.Code, runRec.Body.String())
	}

	logReq := httptest.NewRequest(http.MethodGet, "/v1/logs/events?since=0&limit=100", nil)
	logRec := httptest.NewRecorder()
	router.ServeHTTP(logRec, logReq)
	if logRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", logRec.Code, logRec.Body.String())
	}

	var response struct {
		Events []struct {
			Type string `json:"type"`
		} `json:"events"`
		LatestSequence int64 `json:"latest_sequence"`
	}
	if err := json.Unmarshal(logRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal logs response: %v", err)
	}
	if response.LatestSequence <= 0 {
		t.Fatalf("expected latest_sequence > 0, got %d", response.LatestSequence)
	}
	if len(response.Events) == 0 {
		t.Fatal("expected at least one log event")
	}
}

func TestLogsEndpointIncludesPayloadWhenEnabled(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Port:              "0",
		MemoryFile:        filepath.Join(t.TempDir(), "memory.jsonl"),
		GeminiResponder:   staticResponder{response: "ok"},
		LogAPIEnabled:     true,
		LogEventsEnabled:  true,
		LogIncludePayload: true,
	}
	router := NewRouter(cfg)

	raw := []byte(`{"message":"hello payload","maxSteps":2}`)
	runReq := httptest.NewRequest(http.MethodPost, "/v1/agent/run", bytes.NewReader(raw))
	runReq.Header.Set("Content-Type", "application/json")
	runRec := httptest.NewRecorder()
	router.ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", runRec.Code, runRec.Body.String())
	}

	logReq := httptest.NewRequest(http.MethodGet, "/v1/logs/events?since=0&limit=100&type=agent_run_requested", nil)
	logRec := httptest.NewRecorder()
	router.ServeHTTP(logRec, logReq)
	if logRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", logRec.Code, logRec.Body.String())
	}

	if !strings.Contains(logRec.Body.String(), "hello payload") {
		t.Fatalf("expected payload message in logs when enabled, got %s", logRec.Body.String())
	}
}

func TestLogsEndpointIncludesFullLLMAndToolPayloadsByDefault(t *testing.T) {
	t.Parallel()

	emailConnector := &capturingEmailConnector{}
	connectorRegistry := connectors.NewRegistry()
	if err := connectorRegistry.Register(fakeWebSearch{}); err != nil {
		t.Fatalf("register web-search connector: %v", err)
	}
	if err := connectorRegistry.Register(emailConnector); err != nil {
		t.Fatalf("register email connector: %v", err)
	}

	cfg := LoadConfigFromViper(viper.New())
	cfg.Port = "0"
	cfg.MemoryFile = filepath.Join(t.TempDir(), "memory.jsonl")
	cfg.GeminiResponder = staticResponder{response: "full llm response"}
	cfg.ConnectorRegistry = connectorRegistry
	router := NewRouter(cfg)

	enableEmail := []byte(`{"enabled":true}`)
	enableEmailReq := httptest.NewRequest(http.MethodPatch, "/v1/extensions/email", bytes.NewReader(enableEmail))
	enableEmailReq.Header.Set("Content-Type", "application/json")
	enableEmailRec := httptest.NewRecorder()
	router.ServeHTTP(enableEmailRec, enableEmailReq)
	if enableEmailRec.Code != http.StatusOK {
		t.Fatalf("enable email extension failed: status=%d body=%s", enableEmailRec.Code, enableEmailRec.Body.String())
	}

	runPayload := []byte(`{"message":"search sprint planning updates and check email for invoices","googleAccessToken":"token-abc","maxSteps":5}`)
	runReq := httptest.NewRequest(http.MethodPost, "/v1/agent/run", bytes.NewReader(runPayload))
	runReq.Header.Set("Content-Type", "application/json")
	runRec := httptest.NewRecorder()
	router.ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", runRec.Code, runRec.Body.String())
	}

	logReq := httptest.NewRequest(http.MethodGet, "/v1/logs/events?since=0&limit=200", nil)
	logRec := httptest.NewRecorder()
	router.ServeHTTP(logRec, logReq)
	if logRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", logRec.Code, logRec.Body.String())
	}

	var response struct {
		Events []observability.AuditEvent `json:"events"`
	}
	if err := json.Unmarshal(logRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	var llmRequested, llmSucceeded, webSearchSucceeded, emailSucceeded *observability.AuditEvent
	for i := range response.Events {
		event := &response.Events[i]
		switch {
		case event.Type == observability.EventLLMRequested:
			llmRequested = event
		case event.Type == observability.EventLLMSucceeded:
			llmSucceeded = event
		case event.Type == observability.EventToolSucceeded && event.Tool == "web-search":
			webSearchSucceeded = event
		case event.Type == observability.EventToolSucceeded && event.Tool == "email":
			emailSucceeded = event
		}
	}

	if llmRequested == nil || llmSucceeded == nil || webSearchSucceeded == nil || emailSucceeded == nil {
		t.Fatalf("missing expected events in logs response")
	}
	if !strings.Contains(llmRequested.Metadata["prompt"], "search sprint planning updates and check email for invoices") {
		t.Fatalf("expected full llm prompt in metadata, got %q", llmRequested.Metadata["prompt"])
	}
	if llmSucceeded.Metadata["response"] != "full llm response" {
		t.Fatalf("expected llm response payload in metadata, got %q", llmSucceeded.Metadata["response"])
	}
	if !strings.Contains(webSearchSucceeded.Metadata["tool_output"], "web-search summary") {
		t.Fatalf("expected web-search output details in metadata, got %q", webSearchSucceeded.Metadata["tool_output"])
	}
	if !strings.Contains(emailSucceeded.Metadata["tool_output"], "Invoice") {
		t.Fatalf("expected email output details in metadata, got %q", emailSucceeded.Metadata["tool_output"])
	}
}

func TestLogsEndpointRejectsInvalidQueryValues(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Port:             "0",
		MemoryFile:       filepath.Join(t.TempDir(), "memory.jsonl"),
		GeminiResponder:  staticResponder{response: "ok"},
		LogAPIEnabled:    true,
		LogEventsEnabled: true,
	}
	router := NewRouter(cfg)

	testCases := []struct {
		name          string
		path          string
		expectedError string
	}{
		{
			name:          "invalid since value",
			path:          "/v1/logs/events?since=bad",
			expectedError: "since must be a non-negative integer",
		},
		{
			name:          "negative since value",
			path:          "/v1/logs/events?since=-1",
			expectedError: "since must be a non-negative integer",
		},
		{
			name:          "invalid limit value",
			path:          "/v1/logs/events?limit=bad",
			expectedError: "limit must be a positive integer",
		},
		{
			name:          "non-positive limit value",
			path:          "/v1/logs/events?limit=0",
			expectedError: "limit must be a positive integer",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.expectedError) {
				t.Fatalf("expected error %q in response body %s", tc.expectedError, rec.Body.String())
			}
		})
	}
}

func TestLogsEndpointDefaultsLimitAndOrdersBySequence(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Port:             "0",
		MemoryFile:       filepath.Join(t.TempDir(), "memory.jsonl"),
		GeminiResponder:  staticResponder{response: "ok"},
		LogAPIEnabled:    true,
		LogEventsEnabled: true,
	}
	router := NewRouter(cfg)

	raw := []byte(`{"message":"hello","maxSteps":1}`)
	for i := 0; i < 120; i++ {
		runReq := httptest.NewRequest(http.MethodPost, "/v1/agent/run", bytes.NewReader(raw))
		runReq.Header.Set("Content-Type", "application/json")
		runRec := httptest.NewRecorder()
		router.ServeHTTP(runRec, runReq)
		if runRec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", runRec.Code, runRec.Body.String())
		}
	}

	logReq := httptest.NewRequest(http.MethodGet, "/v1/logs/events?since=0", nil)
	logRec := httptest.NewRecorder()
	router.ServeHTTP(logRec, logReq)
	if logRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", logRec.Code, logRec.Body.String())
	}

	var response struct {
		Events []struct {
			Sequence int64 `json:"sequence"`
		} `json:"events"`
		LatestSequence int64 `json:"latest_sequence"`
	}
	if err := json.Unmarshal(logRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal logs response: %v", err)
	}

	if response.LatestSequence <= int64(logEventsDefaultLimit) {
		t.Fatalf("expected latest_sequence greater than default limit, got %d", response.LatestSequence)
	}
	if len(response.Events) != logEventsDefaultLimit {
		t.Fatalf("expected default %d events, got %d", logEventsDefaultLimit, len(response.Events))
	}
	for i := 1; i < len(response.Events); i++ {
		if response.Events[i-1].Sequence >= response.Events[i].Sequence {
			t.Fatalf("expected strictly increasing event sequence, got %d then %d", response.Events[i-1].Sequence, response.Events[i].Sequence)
		}
	}
}

func TestParseLogEventsLimit(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		raw       string
		expected  int
		expectErr bool
	}{
		{name: "default when empty", raw: "", expected: logEventsDefaultLimit},
		{name: "valid explicit limit", raw: "50", expected: 50},
		{name: "trimmed valid limit", raw: " 75 ", expected: 75},
		{name: "clamps above max", raw: "5000", expected: logEventsMaxLimit},
		{name: "rejects zero", raw: "0", expectErr: true},
		{name: "rejects negative", raw: "-2", expectErr: true},
		{name: "rejects non-numeric", raw: "bad", expectErr: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			limit, err := parseLogEventsLimit(tc.raw)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for input %q", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error for input %q, got %v", tc.raw, err)
			}
			if limit != tc.expected {
				t.Fatalf("expected limit %d, got %d", tc.expected, limit)
			}
		})
	}
}
