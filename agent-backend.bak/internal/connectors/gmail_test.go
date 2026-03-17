package connectors

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"worldagent/agent-backend/internal/config"
)

type stubGmailHTTPClient struct {
	do func(req *http.Request) (*http.Response, error)
}

func (s stubGmailHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return s.do(req)
}

func TestNewGmailConnectorRequiresOAuthConfig(t *testing.T) {
	t.Parallel()

	_, err := NewGmailConnector(config.GoogleOAuthConfig{})
	if err == nil || !strings.Contains(err.Error(), "GOOGLE_CLIENT_ID is required") {
		t.Fatalf("expected missing client id error, got %v", err)
	}
}

func TestGmailConnectorListMessagesUsesAccessToken(t *testing.T) {
	t.Parallel()

	connector, err := NewGmailConnector(config.GoogleOAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://example.com/callback",
	}, WithGmailHTTPClient(stubGmailHTTPClient{
		do: func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET request, got %s", req.Method)
			}
			if req.Header.Get("Authorization") != "Bearer token-123" {
				t.Fatalf("expected bearer token header, got %q", req.Header.Get("Authorization"))
			}
			if req.URL.Query().Get("q") != "is:unread" {
				t.Fatalf("expected query filter, got %q", req.URL.Query().Get("q"))
			}
			if req.URL.Query().Get("maxResults") != "2" {
				t.Fatalf("expected maxResults query, got %q", req.URL.Query().Get("maxResults"))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"messages": [
						{"id": "m1", "threadId": "t1"},
						{"id": "m2", "threadId": "t2"}
					]
				}`)),
			}, nil
		},
	}))
	if err != nil {
		t.Fatalf("new gmail connector: %v", err)
	}

	messages, err := connector.ListMessages(context.Background(), ListMessagesRequest{
		AccessToken: "token-123",
		Query:       "is:unread",
		MaxResults:  2,
	})
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].ID != "m1" || messages[1].ID != "m2" {
		t.Fatalf("unexpected message IDs: %+v", messages)
	}
}

func TestGmailConnectorListMessagesRequiresToken(t *testing.T) {
	t.Parallel()

	connector, err := NewGmailConnector(config.GoogleOAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://example.com/callback",
	})
	if err != nil {
		t.Fatalf("new gmail connector: %v", err)
	}

	_, err = connector.ListMessages(context.Background(), ListMessagesRequest{})
	if err == nil || !strings.Contains(err.Error(), "gmail access token is required") {
		t.Fatalf("expected missing token error, got %v", err)
	}
}

func TestGmailConnectorGetMessageParsesMetadata(t *testing.T) {
	t.Parallel()

	connector, err := NewGmailConnector(config.GoogleOAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://example.com/callback",
	}, WithGmailHTTPClient(stubGmailHTTPClient{
		do: func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET request, got %s", req.Method)
			}
			if req.Header.Get("Authorization") != "Bearer token-123" {
				t.Fatalf("expected bearer token header, got %q", req.Header.Get("Authorization"))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"id": "m1",
					"threadId": "t1",
					"snippet": "hello world",
					"payload": {
						"headers": [
							{"name": "Subject", "value": "Weekly update"},
							{"name": "From", "value": "news@example.com"}
						]
					}
				}`)),
			}, nil
		},
	}))
	if err != nil {
		t.Fatalf("new gmail connector: %v", err)
	}

	message, err := connector.GetMessage(context.Background(), GetMessageRequest{
		AccessToken: "token-123",
		MessageID:   "m1",
	})
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if message.Subject != "Weekly update" || message.From != "news@example.com" {
		t.Fatalf("expected parsed headers, got %+v", message)
	}
	if message.Snippet != "hello world" {
		t.Fatalf("expected snippet, got %+v", message)
	}
}

func TestGmailConnectorSendMessageEncodesRawPayload(t *testing.T) {
	t.Parallel()

	connector, err := NewGmailConnector(config.GoogleOAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://example.com/callback",
	}, WithGmailHTTPClient(stubGmailHTTPClient{
		do: func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST request, got %s", req.Method)
			}
			if req.Header.Get("Authorization") != "Bearer token-123" {
				t.Fatalf("expected bearer token header, got %q", req.Header.Get("Authorization"))
			}
			rawRequestBody, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			var payload map[string]string
			if err := json.Unmarshal(rawRequestBody, &payload); err != nil {
				t.Fatalf("decode send payload: %v", err)
			}
			decodedRaw, err := base64.RawURLEncoding.DecodeString(payload["raw"])
			if err != nil {
				t.Fatalf("decode raw message: %v", err)
			}
			rawMessage := string(decodedRaw)
			if !strings.Contains(rawMessage, "To: reader@example.com") {
				t.Fatalf("expected recipient in raw payload, got %q", rawMessage)
			}
			if !strings.Contains(rawMessage, "Subject: Hello") {
				t.Fatalf("expected subject in raw payload, got %q", rawMessage)
			}
			if !strings.Contains(rawMessage, "Message body") {
				t.Fatalf("expected body in raw payload, got %q", rawMessage)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"id":"sent-1","threadId":"thread-1"}`)),
			}, nil
		},
	}))
	if err != nil {
		t.Fatalf("new gmail connector: %v", err)
	}

	response, err := connector.SendMessage(context.Background(), SendMessageRequest{
		AccessToken: "token-123",
		To:          []string{"reader@example.com"},
		Subject:     "Hello",
		Body:        "Message body",
	})
	if err != nil {
		t.Fatalf("send message: %v", err)
	}
	if response.ID != "sent-1" || response.ThreadID != "thread-1" {
		t.Fatalf("unexpected send response: %+v", response)
	}
}

func TestGmailConnectorSendMessageRequiresRecipient(t *testing.T) {
	t.Parallel()

	connector, err := NewGmailConnector(config.GoogleOAuthConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://example.com/callback",
	})
	if err != nil {
		t.Fatalf("new gmail connector: %v", err)
	}

	_, err = connector.SendMessage(context.Background(), SendMessageRequest{
		AccessToken: "token-123",
		Body:        "hello",
	})
	if err == nil || !strings.Contains(err.Error(), "gmail recipient is required") {
		t.Fatalf("expected recipient error, got %v", err)
	}
}
