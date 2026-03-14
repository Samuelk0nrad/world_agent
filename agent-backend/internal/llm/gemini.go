package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"worldagent/agent-backend/internal/config"
	"worldagent/agent-backend/internal/connectors"
)

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com"

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type GeminiClient struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient httpClient
}

type GeminiOption func(*GeminiClient)

func WithGeminiBaseURL(baseURL string) GeminiOption {
	return func(client *GeminiClient) {
		trimmed := strings.TrimSpace(baseURL)
		if trimmed != "" {
			client.baseURL = strings.TrimRight(trimmed, "/")
		}
	}
}

func WithGeminiHTTPClient(httpClient httpClient) GeminiOption {
	return func(client *GeminiClient) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

func NewGeminiClient(cfg config.GeminiConfig, options ...GeminiOption) (*GeminiClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client := &GeminiClient{
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		baseURL:    defaultGeminiBaseURL,
		httpClient: http.DefaultClient,
	}
	for _, option := range options {
		option(client)
	}
	return client, nil
}

func NewGeminiClientFromEnv(options ...GeminiOption) (*GeminiClient, error) {
	cfg := config.LoadGeminiConfig()
	return NewGeminiClient(cfg, options...)
}

func (c *GeminiClient) Generate(ctx context.Context, prompt string) (string, error) {
	trimmedPrompt := strings.TrimSpace(prompt)
	if trimmedPrompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	requestBody := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": trimmedPrompt},
				},
			},
		},
	}
	rawBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal gemini request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", c.baseURL, url.PathEscape(c.model), url.QueryEscape(c.apiKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return "", fmt.Errorf("create gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call gemini API: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read gemini response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("gemini API request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var decoded struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return "", fmt.Errorf("decode gemini response: %w", err)
	}
	if len(decoded.Candidates) == 0 || len(decoded.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini response does not contain any text candidates")
	}

	text := strings.TrimSpace(decoded.Candidates[0].Content.Parts[0].Text)
	if text == "" {
		return "", fmt.Errorf("gemini response text is empty")
	}

	return text, nil
}

func (c *GeminiClient) ID() string {
	return connectors.GeminiConnectorID
}

var _ Responder = (*GeminiClient)(nil)
var _ connectors.TextGenerationConnector = (*GeminiClient)(nil)
