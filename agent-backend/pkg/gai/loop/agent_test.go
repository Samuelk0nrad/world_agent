package loop_test

import (
	"context"
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
	if msg.Role != "agent" || msg.Text != "ok" {
		t.Fatalf("unexpected message: %+v", *msg)
	}

	if len(agent.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(agent.Messages))
	}
	if agent.Messages[0].Role != "user" || agent.Messages[0].Text != "hello" {
		t.Fatalf("unexpected first message: %+v", agent.Messages[0])
	}
	if agent.Messages[1].Role != "agent" || agent.Messages[1].Text != "ok" {
		t.Fatalf("unexpected second message: %+v", agent.Messages[1])
	}

	combined := model.lastReq.CombinedPrompt()
	if !strings.Contains(combined, "system prompt") {
		t.Fatalf("combined prompt missing system prompt: %q", combined)
	}
	if !strings.Contains(combined, "0 (user):") {
		t.Fatalf("combined prompt missing message index and role: %q", combined)
	}
	if strings.Count(combined, "hello") != 1 {
		t.Fatalf("expected user prompt once in combined prompt, got %q", combined)
	}
}
