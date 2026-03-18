package loop_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-backend/pkg/gai/ai"
	"agent-backend/pkg/gai/loop"
)

type fakeModel struct {
	lastReq ai.AIRequest
}

func (f *fakeModel) Name() string {
	return "fake"
}

func (f *fakeModel) Generate(_ context.Context, req ai.AIRequest) (*ai.AIResponse, error) {
	f.lastReq = req
	return &ai.AIResponse{Text: "ok"}, nil
}

func TestFollowUpAppendsMessagesAndBuildsPrompt(t *testing.T) {
	model := &fakeModel{}
	agent := loop.NewAgent(model, nil, "system prompt")

	msg, err := agent.FollowUp(context.Background(), "hello")
	if err != nil {
		t.Fatalf("FollowUp returned error: %v", err)
	}
	if msg == nil {
		t.Fatalf("expected message, got nil")
	}
	if msg.Role != loop.RoleAssistant || msg.Text != "ok" {
		t.Fatalf("unexpected message: %+v", *msg)
	}

	if len(agent.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(agent.Messages))
	}
	if agent.Messages[0].Role != loop.RoleUser || agent.Messages[0].Text != "hello" {
		t.Fatalf("unexpected first message: %+v", agent.Messages[0])
	}
	if agent.Messages[1].Role != loop.RoleAssistant || agent.Messages[1].Text != "ok" {
		t.Fatalf("unexpected second message: %+v", agent.Messages[1])
	}

	if !strings.Contains(model.lastReq.SystemPrompt, "system prompt") {
		t.Fatalf("system prompt missing from request: %q", model.lastReq.SystemPrompt)
	}
	if strings.Count(model.lastReq.SystemPrompt, "hello") != 1 {
		t.Fatalf("expected user prompt once in model request, got %q", model.lastReq.SystemPrompt)
	}
}

func TestFollowUpValidation(t *testing.T) {
	t.Run("nil agent", func(t *testing.T) {
		var agent *loop.Agent
		_, err := agent.FollowUp(context.Background(), "hello")
		if !errors.Is(err, loop.ErrNilAgent) {
			t.Fatalf("expected ErrNilAgent, got %v", err)
		}
	})

	t.Run("missing model", func(t *testing.T) {
		agent := loop.NewAgent(nil, nil, "system")
		_, err := agent.FollowUp(context.Background(), "hello")
		if !errors.Is(err, loop.ErrModelNotConfigured) {
			t.Fatalf("expected ErrModelNotConfigured, got %v", err)
		}
	})

	t.Run("empty prompt", func(t *testing.T) {
		agent := loop.NewAgent(&fakeModel{}, nil, "system")
		_, err := agent.FollowUp(context.Background(), "   ")
		if !errors.Is(err, loop.ErrEmptyPrompt) {
			t.Fatalf("expected ErrEmptyPrompt, got %v", err)
		}
	})
}
