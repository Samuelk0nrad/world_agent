package connectors

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"worldagent/agent-backend/internal/config"
)

const defaultGmailBaseURL = "https://gmail.googleapis.com"

type gmailHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type GmailConnector struct {
	oauthConfig config.GoogleOAuthConfig
	baseURL     string
	httpClient  gmailHTTPClient
}

type GmailOption func(*GmailConnector)

func WithGmailBaseURL(baseURL string) GmailOption {
	return func(connector *GmailConnector) {
		trimmed := strings.TrimSpace(baseURL)
		if trimmed != "" {
			connector.baseURL = strings.TrimRight(trimmed, "/")
		}
	}
}

func WithGmailHTTPClient(httpClient gmailHTTPClient) GmailOption {
	return func(connector *GmailConnector) {
		if httpClient != nil {
			connector.httpClient = httpClient
		}
	}
}

func NewGmailConnector(cfg config.GoogleOAuthConfig, options ...GmailOption) (*GmailConnector, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	connector := &GmailConnector{
		oauthConfig: cfg,
		baseURL:     defaultGmailBaseURL,
		httpClient:  http.DefaultClient,
	}
	for _, option := range options {
		option(connector)
	}
	return connector, nil
}

func NewGmailConnectorFromEnv(options ...GmailOption) (*GmailConnector, error) {
	cfg := config.LoadGoogleOAuthConfig()
	return NewGmailConnector(cfg, options...)
}

func (c *GmailConnector) ID() string {
	return GmailConnectorID
}

func (c *GmailConnector) ListMessages(ctx context.Context, request ListMessagesRequest) ([]EmailMessage, error) {
	if err := c.validateConfig(); err != nil {
		return nil, err
	}
	accessToken := strings.TrimSpace(request.AccessToken)
	if accessToken == "" {
		return nil, fmt.Errorf("gmail access token is required")
	}

	endpoint, err := url.Parse(c.baseURL + "/gmail/v1/users/me/messages")
	if err != nil {
		return nil, fmt.Errorf("build gmail list endpoint: %w", err)
	}

	values := endpoint.Query()
	if request.MaxResults > 0 {
		values.Set("maxResults", fmt.Sprintf("%d", request.MaxResults))
	}
	if strings.TrimSpace(request.Query) != "" {
		values.Set("q", strings.TrimSpace(request.Query))
	}
	endpoint.RawQuery = values.Encode()

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create gmail list request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+accessToken)

	responseBody, err := c.do(httpRequest, "list gmail messages")
	if err != nil {
		return nil, err
	}

	var payload struct {
		Messages []struct {
			ID       string `json:"id"`
			ThreadID string `json:"threadId"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("decode gmail list response: %w", err)
	}

	out := make([]EmailMessage, 0, len(payload.Messages))
	for _, item := range payload.Messages {
		out = append(out, EmailMessage{
			ID:       strings.TrimSpace(item.ID),
			ThreadID: strings.TrimSpace(item.ThreadID),
		})
	}
	return out, nil
}

func (c *GmailConnector) GetMessage(ctx context.Context, request GetMessageRequest) (EmailMessage, error) {
	if err := c.validateConfig(); err != nil {
		return EmailMessage{}, err
	}
	accessToken := strings.TrimSpace(request.AccessToken)
	if accessToken == "" {
		return EmailMessage{}, fmt.Errorf("gmail access token is required")
	}
	messageID := strings.TrimSpace(request.MessageID)
	if messageID == "" {
		return EmailMessage{}, fmt.Errorf("gmail message id is required")
	}

	endpoint := fmt.Sprintf("%s/gmail/v1/users/me/messages/%s?format=metadata&metadataHeaders=Subject&metadataHeaders=From", c.baseURL, url.PathEscape(messageID))
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return EmailMessage{}, fmt.Errorf("create gmail get request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+accessToken)

	responseBody, err := c.do(httpRequest, "get gmail message")
	if err != nil {
		return EmailMessage{}, err
	}

	var payload struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
		Snippet  string `json:"snippet"`
		Payload  struct {
			Headers []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"headers"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return EmailMessage{}, fmt.Errorf("decode gmail message response: %w", err)
	}

	message := EmailMessage{
		ID:       strings.TrimSpace(payload.ID),
		ThreadID: strings.TrimSpace(payload.ThreadID),
		Snippet:  strings.TrimSpace(payload.Snippet),
	}
	for _, header := range payload.Payload.Headers {
		switch strings.ToLower(strings.TrimSpace(header.Name)) {
		case "subject":
			message.Subject = strings.TrimSpace(header.Value)
		case "from":
			message.From = strings.TrimSpace(header.Value)
		}
	}
	return message, nil
}

func (c *GmailConnector) SendMessage(ctx context.Context, request SendMessageRequest) (SendMessageResponse, error) {
	if err := c.validateConfig(); err != nil {
		return SendMessageResponse{}, err
	}
	accessToken := strings.TrimSpace(request.AccessToken)
	if accessToken == "" {
		return SendMessageResponse{}, fmt.Errorf("gmail access token is required")
	}
	if len(request.To) == 0 {
		return SendMessageResponse{}, fmt.Errorf("gmail recipient is required")
	}
	body := strings.TrimSpace(request.Body)
	if body == "" {
		return SendMessageResponse{}, fmt.Errorf("gmail body is required")
	}

	rawMessage := formatRawMessage(request.To, request.Subject, body)
	encoded := base64.RawURLEncoding.EncodeToString([]byte(rawMessage))

	requestPayload := map[string]string{"raw": encoded}
	rawPayload, err := json.Marshal(requestPayload)
	if err != nil {
		return SendMessageResponse{}, fmt.Errorf("marshal gmail send request: %w", err)
	}

	endpoint := c.baseURL + "/gmail/v1/users/me/messages/send"
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawPayload))
	if err != nil {
		return SendMessageResponse{}, fmt.Errorf("create gmail send request: %w", err)
	}
	httpRequest.Header.Set("Authorization", "Bearer "+accessToken)
	httpRequest.Header.Set("Content-Type", "application/json")

	responseBody, err := c.do(httpRequest, "send gmail message")
	if err != nil {
		return SendMessageResponse{}, err
	}

	var payload struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return SendMessageResponse{}, fmt.Errorf("decode gmail send response: %w", err)
	}

	return SendMessageResponse{
		ID:       strings.TrimSpace(payload.ID),
		ThreadID: strings.TrimSpace(payload.ThreadID),
	}, nil
}

func (c *GmailConnector) validateConfig() error {
	if c == nil {
		return fmt.Errorf("gmail connector is not configured")
	}
	if err := c.oauthConfig.Validate(); err != nil {
		return fmt.Errorf("gmail oauth config is invalid: %w", err)
	}
	return nil
}

func (c *GmailConnector) do(req *http.Request, action string) ([]byte, error) {
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", action, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read gmail response: %w", err)
	}

	if response.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("%s failed with status %d: %s", action, response.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func formatRawMessage(to []string, subject, body string) string {
	recipients := make([]string, 0, len(to))
	for _, recipient := range to {
		trimmed := strings.TrimSpace(recipient)
		if trimmed != "" {
			recipients = append(recipients, trimmed)
		}
	}

	var builder strings.Builder
	builder.WriteString("To: ")
	builder.WriteString(strings.Join(recipients, ", "))
	builder.WriteString("\r\n")
	builder.WriteString("Subject: ")
	builder.WriteString(strings.TrimSpace(subject))
	builder.WriteString("\r\n")
	builder.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	builder.WriteString("\r\n")
	builder.WriteString(strings.TrimSpace(body))
	return builder.String()
}

var _ EmailConnector = (*GmailConnector)(nil)
