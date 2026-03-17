package ai

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestSliceEventStreamIteratesInOrder(t *testing.T) {
	t.Parallel()

	stream := NewSliceEventStream(
		MessageStartEvent{Model: DefaultGeminiModel()},
		ContentDeltaEvent{Delta: TextContent{Text: "hello"}},
		MessageCompleteEvent{
			Message: Message{
				Role: RoleAssistant,
				Content: []ContentPart{
					TextContent{Text: "hello"},
				},
			},
		},
	)
	defer stream.Close()

	first, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("next first event: %v", err)
	}
	if first.Kind() != AssistantEventKindMessageStart {
		t.Fatalf("expected message_start, got %q", first.Kind())
	}

	second, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("next second event: %v", err)
	}
	if second.Kind() != AssistantEventKindContentDelta {
		t.Fatalf("expected content_delta, got %q", second.Kind())
	}

	third, err := stream.Next(context.Background())
	if err != nil {
		t.Fatalf("next third event: %v", err)
	}
	if third.Kind() != AssistantEventKindMessageComplete {
		t.Fatalf("expected message_complete, got %q", third.Kind())
	}

	_, err = stream.Next(context.Background())
	if err != io.EOF {
		t.Fatalf("expected EOF after stream consumption, got %v", err)
	}
}

func TestSliceEventStreamReturnsContextError(t *testing.T) {
	t.Parallel()

	stream := NewSliceEventStream(MessageStartEvent{Model: DefaultGeminiModel()})
	defer stream.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := stream.Next(ctx)
	if err != context.Canceled {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestSliceEventStreamCloseReturnsEOF(t *testing.T) {
	t.Parallel()

	stream := NewSliceEventStream(MessageStartEvent{Model: DefaultGeminiModel()})
	if err := stream.Close(); err != nil {
		t.Fatalf("close stream: %v", err)
	}

	_, err := stream.Next(context.Background())
	if err != io.EOF {
		t.Fatalf("expected EOF after close, got %v", err)
	}
}

func TestSliceEventStreamReturnsErrorForNilEvent(t *testing.T) {
	t.Parallel()

	stream := NewSliceEventStream(nil)
	defer stream.Close()

	_, err := stream.Next(context.Background())
	if err == nil {
		t.Fatal("expected nil event error")
	}
	if !strings.Contains(err.Error(), "assistant event 0 is nil") {
		t.Fatalf("expected nil event index error, got %v", err)
	}
}
