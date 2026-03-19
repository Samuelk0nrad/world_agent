package loop_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"agent-backend/pkg/gai/ai"
	"agent-backend/pkg/gai/loop"
)

type fakeModel struct {
	lastReq     ai.AIRequest
	responses   []string
	callCount   int
	errOnCall   int
	generateErr error
}

func (f *fakeModel) Name() string {
	return "fake"
}

func (f *fakeModel) Generate(_ context.Context, req ai.AIRequest) (*ai.AIResponse, error) {
	f.lastReq = req
	f.callCount++
	if f.generateErr != nil && f.errOnCall == f.callCount {
		return nil, f.generateErr
	}
	if len(f.responses) > 0 {
		idx := f.callCount - 1
		if idx >= len(f.responses) {
			idx = len(f.responses) - 1
		}
		return &ai.AIResponse{Text: f.responses[idx]}, nil
	}
	return &ai.AIResponse{Text: "ok"}, nil
}

func TestFollowUpAppendsMessagesAndBuildsPrompt(t *testing.T) {
	model := &fakeModel{}
	agent := loop.NewAgent(model, nil, "system prompt")

	msg, err := agent.FollowUp(context.Background(), "hello")
	if err != nil {
		t.Fatalf("FollowUp returned error: %v", err)
	}
	if msg == "" {
		t.Fatalf("expected message, got empty string")
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
	if strings.Contains(model.lastReq.SystemPrompt, "<system") {
		t.Fatalf("did not expect system prompt to be stored as conversation message: %q", model.lastReq.SystemPrompt)
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

type fakeTool struct {
	name string
	resp string
	err  error
}

func (t *fakeTool) Name() string {
	return t.name
}

func (t *fakeTool) Description() string {
	return "fake tool"
}

func (t *fakeTool) Params() string {
	return "{}"
}

func (t *fakeTool) Function(_ *loop.ToolRequest) (*loop.ToolResponse, error) {
	if t.err != nil {
		return nil, t.err
	}
	return &loop.ToolResponse{Text: t.resp}, nil
}

func TestFollowUpReturnsUnknownToolErrorInTranscript(t *testing.T) {
	model := &fakeModel{
		responses: []string{`{"id":"missing","type":"function","arguments":"hello"}`},
	}
	agent := loop.NewAgent(model, nil, "system")

	msg, err := agent.FollowUp(context.Background(), "run tool")
	if err != nil {
		t.Fatalf("FollowUp returned unexpected error: %v", err)
	}
	if !strings.Contains(msg, "tool not found: missing") {
		t.Fatalf("expected unknown tool text in response, got %q", msg)
	}
	if len(agent.Messages) != 3 {
		t.Fatalf("expected user, assistant, tool messages; got %d", len(agent.Messages))
	}
	if agent.Messages[2].Role != loop.RoleTool {
		t.Fatalf("expected tool message role, got %v", agent.Messages[2].Role)
	}
}

func TestFollowUpStopsWhenToolErrors(t *testing.T) {
	model := &fakeModel{
		responses: []string{`{"id":"boom","type":"function","arguments":"hello"}`},
	}
	tool := &fakeTool{name: "boom", err: errors.New("boom failure")}
	agent := loop.NewAgent(model, []loop.Tool{tool}, "system")

	msg, err := agent.FollowUp(context.Background(), "run tool")
	if err != nil {
		t.Fatalf("FollowUp returned unexpected error: %v", err)
	}
	if !strings.Contains(msg, "boom failure") {
		t.Fatalf("expected tool error in transcript, got %q", msg)
	}
	if model.callCount != 1 {
		t.Fatalf("expected one model call when tool fails, got %d", model.callCount)
	}
}

func TestFollowUpContinuesAfterSuccessfulTool(t *testing.T) {
	arguments, _ := json.Marshal("hello")
	model := &fakeModel{
		responses: []string{
			`{"id":"echo","type":"function","arguments":` + string(arguments) + `}`,
			"final answer",
		},
	}
	tool := &fakeTool{name: "echo", resp: "hello"}
	agent := loop.NewAgent(model, []loop.Tool{tool}, "system")

	msg, err := agent.FollowUp(context.Background(), "use tool")
	if err != nil {
		t.Fatalf("FollowUp returned unexpected error: %v", err)
	}
	if !strings.Contains(msg, "Tool echo hello") || !strings.Contains(msg, "final answer") {
		t.Fatalf("expected tool and final answer in transcript, got %q", msg)
	}
	if model.callCount != 2 {
		t.Fatalf("expected two model calls, got %d", model.callCount)
	}
}

func TestFollowUpRespectsMaxLoopIterations(t *testing.T) {
	model := &fakeModel{
		responses: []string{`{"id":"echo","type":"function","arguments":"x"}`},
	}
	tool := &fakeTool{name: "echo", resp: "x"}
	agent := loop.NewAgent(model, []loop.Tool{tool}, "system")
	agent.MaxLoopIterations = 1

	_, err := agent.FollowUp(context.Background(), "loop")
	if !errors.Is(err, loop.ErrMaxIterations) {
		t.Fatalf("expected ErrMaxIterations, got %v", err)
	}
}
