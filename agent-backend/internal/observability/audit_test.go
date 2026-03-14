package observability

import "testing"

func TestInMemoryAuditSinkAssignsSequenceAndLatest(t *testing.T) {
	t.Parallel()

	sink := NewInMemoryAuditSink(10)
	_ = sink.Record(nil, AuditEvent{Type: EventToolAttempted, Tool: "web-search"})
	_ = sink.Record(nil, AuditEvent{Type: EventToolSucceeded, Tool: "web-search"})

	events := sink.EventsSince(0, 10, "")
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Fatalf("unexpected sequences: %d %d", events[0].Sequence, events[1].Sequence)
	}
	if sink.LatestSequence() != 2 {
		t.Fatalf("expected latest sequence 2, got %d", sink.LatestSequence())
	}
}

func TestInMemoryAuditSinkCapacityAndFiltering(t *testing.T) {
	t.Parallel()

	sink := NewInMemoryAuditSink(2)
	_ = sink.Record(nil, AuditEvent{Type: EventToolAttempted, Tool: "web-search"})
	_ = sink.Record(nil, AuditEvent{Type: EventLLMRequested, Tool: "llm"})
	_ = sink.Record(nil, AuditEvent{Type: EventLLMSucceeded, Tool: "llm"})

	events := sink.EventsSince(0, 10, "")
	if len(events) != 2 {
		t.Fatalf("expected 2 events after capacity trimming, got %d", len(events))
	}
	if events[0].Sequence != 2 || events[1].Sequence != 3 {
		t.Fatalf("expected retained sequences 2 and 3, got %d and %d", events[0].Sequence, events[1].Sequence)
	}

	filtered := sink.EventsSince(0, 10, EventLLMSucceeded)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 llm success event, got %d", len(filtered))
	}
	if filtered[0].Type != EventLLMSucceeded {
		t.Fatalf("unexpected filtered event type: %s", filtered[0].Type)
	}
}
