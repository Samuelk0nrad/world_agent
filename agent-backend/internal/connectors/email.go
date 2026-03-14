package connectors

import (
	"context"
	"fmt"
	"strings"
)

const GmailConnectorID = "gmail"

type EmailMessage struct {
	ID       string
	ThreadID string
	Snippet  string
	Subject  string
	From     string
}

type ListMessagesRequest struct {
	AccessToken string
	Query       string
	MaxResults  int
}

type GetMessageRequest struct {
	AccessToken string
	MessageID   string
}

type SendMessageRequest struct {
	AccessToken string
	To          []string
	Subject     string
	Body        string
}

type SendMessageResponse struct {
	ID       string
	ThreadID string
}

type EmailConnector interface {
	Connector
	ListMessages(ctx context.Context, request ListMessagesRequest) ([]EmailMessage, error)
	GetMessage(ctx context.Context, request GetMessageRequest) (EmailMessage, error)
	SendMessage(ctx context.Context, request SendMessageRequest) (SendMessageResponse, error)
}

func GetEmailConnector(registry *Registry, id string) (EmailConnector, error) {
	if registry == nil {
		return nil, fmt.Errorf("connector registry is not configured")
	}

	connectorID := normalizeConnectorID(id)
	if connectorID == "" {
		return nil, fmt.Errorf("email connector id is required")
	}

	connector, ok := registry.Get(connectorID)
	if !ok {
		return nil, fmt.Errorf("connector %q is not registered", connectorID)
	}

	emailConnector, ok := connector.(EmailConnector)
	if !ok {
		return nil, fmt.Errorf("connector %q does not implement email operations", connectorID)
	}

	return emailConnector, nil
}

type unavailableEmailConnector struct {
	id     string
	reason error
}

func NewUnavailableEmailConnector(id string, reason error) EmailConnector {
	connectorID := normalizeConnectorID(id)
	if connectorID == "" {
		connectorID = GmailConnectorID
	}
	return unavailableEmailConnector{
		id:     connectorID,
		reason: reason,
	}
}

func (c unavailableEmailConnector) ID() string {
	return c.id
}

func (c unavailableEmailConnector) ListMessages(context.Context, ListMessagesRequest) ([]EmailMessage, error) {
	return nil, c.unavailableError()
}

func (c unavailableEmailConnector) GetMessage(context.Context, GetMessageRequest) (EmailMessage, error) {
	return EmailMessage{}, c.unavailableError()
}

func (c unavailableEmailConnector) SendMessage(context.Context, SendMessageRequest) (SendMessageResponse, error) {
	return SendMessageResponse{}, c.unavailableError()
}

func (c unavailableEmailConnector) unavailableError() error {
	if c.reason == nil {
		return fmt.Errorf("email connector %q is unavailable", c.id)
	}
	return fmt.Errorf("email connector %q is unavailable: %w", c.id, c.reason)
}

type emailAccessTokenContextKey struct{}

func WithEmailAccessToken(ctx context.Context, accessToken string) context.Context {
	trimmed := strings.TrimSpace(accessToken)
	if trimmed == "" {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, emailAccessTokenContextKey{}, trimmed)
}

func EmailAccessTokenFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value := ctx.Value(emailAccessTokenContextKey{})
	token, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(token)
}
