package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"worldagent/agent-backend/internal/config"
	"worldagent/agent-backend/internal/connectors"
)

type stubHTTPClient struct {
	do func(req *http.Request) (*http.Response, error)
}

func (s stubHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return s.do(req)
}

func TestNewGeminiClientImplementsConnectorID(t *testing.T) {
	t.Parallel()

	client, err := NewGeminiClient(config.GeminiConfig{
		APIKey: "api-key",
		Model:  config.DefaultGeminiModel,
	})
	if err != nil {
		t.Fatalf("new gemini client: %v", err)
	}
	if client.ID() != connectors.GeminiConnectorID {
		t.Fatalf("expected connector ID %q, got %q", connectors.GeminiConnectorID, client.ID())
	}
}

func TestGeminiClientGenerateSuccess(t *testing.T) {
	t.Parallel()

	client, err := NewGeminiClient(config.GeminiConfig{
		APIKey: "my-key",
		Model:  "gemini-2.0-flash",
	}, WithGeminiHTTPClient(stubHTTPClient{
		do: func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST request, got %s", req.Method)
			}
			if req.URL.Query().Get("key") != "my-key" {
				t.Fatalf("expected API key in query string, got %q", req.URL.Query().Get("key"))
			}
			if !strings.Contains(req.URL.Path, "/models/gemini-2.0-flash:generateContent") {
				t.Fatalf("unexpected Gemini path: %s", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"candidates": [{
						"content": {
							"parts": [{"text": "  hello from gemini  "}]
						}
					}]
				}`)),
			}, nil
		},
	}))
	if err != nil {
		t.Fatalf("new gemini client: %v", err)
	}

	response, err := client.Generate(context.Background(), "hello")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if response != "hello from gemini" {
		t.Fatalf("expected trimmed response, got %q", response)
	}
}

func TestGeminiClientGenerateReturnsAPIError(t *testing.T) {
	t.Parallel()

	client, err := NewGeminiClient(config.GeminiConfig{
		APIKey: "my-key",
		Model:  config.DefaultGeminiModel,
	}, WithGeminiHTTPClient(stubHTTPClient{
		do: func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader(`upstream unavailable`)),
			}, nil
		},
	}))
	if err != nil {
		t.Fatalf("new gemini client: %v", err)
	}

	_, err = client.Generate(context.Background(), "hello")
	if err == nil || !strings.Contains(err.Error(), "status 502") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestGeminiClientGenerateReturnsTransportError(t *testing.T) {
	t.Parallel()

	client, err := NewGeminiClient(config.GeminiConfig{
		APIKey: "my-key",
		Model:  config.DefaultGeminiModel,
	}, WithGeminiHTTPClient(stubHTTPClient{
		do: func(*http.Request) (*http.Response, error) {
			return nil, errors.New("dial timeout")
		},
	}))
	if err != nil {
		t.Fatalf("new gemini client: %v", err)
	}

	_, err = client.Generate(context.Background(), "hello")
	if err == nil || !strings.Contains(err.Error(), "call gemini API: dial timeout") {
		t.Fatalf("expected wrapped transport error, got %v", err)
	}
}
