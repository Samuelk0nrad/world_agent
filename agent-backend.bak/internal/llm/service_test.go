package llm

import (
	"context"
	"errors"
	"testing"
)

type testResponder struct {
	response string
	err      error
}

func (r testResponder) Generate(context.Context, string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return r.response, nil
}

func TestGenerateAgentResponseRequiresResponder(t *testing.T) {
	t.Parallel()

	_, err := GenerateAgentResponse(context.Background(), nil, "hello")
	if !errors.Is(err, ErrResponderUnavailable) {
		t.Fatalf("expected explicit missing responder error, got %v", err)
	}
}

func TestGenerateAgentResponseReturnsResponderError(t *testing.T) {
	t.Parallel()

	_, err := GenerateAgentResponse(context.Background(), testResponder{err: errors.New("boom")}, "hello")
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom error, got %v", err)
	}
}

func TestNewStaticErrorResponderReturnsConfiguredError(t *testing.T) {
	t.Parallel()

	responder := NewStaticErrorResponder(errors.New("config failure"))
	_, err := GenerateAgentResponse(context.Background(), responder, "hello")
	if err == nil || err.Error() != "config failure" {
		t.Fatalf("expected static responder error, got %v", err)
	}
}
