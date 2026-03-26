package memory

import (
	"encoding/json"
	"testing"
)

func TestEntryUnmarshalLegacyCreatedAt(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"id":"mem-1","source":"test","content":"hello","createdAt":"2025-01-01T00:00:00Z","sequence":1}`)
	var entry Entry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if entry.CreatedAt.IsZero() {
		t.Fatalf("expected created_at to be populated")
	}
	if entry.Sequence != 1 {
		t.Fatalf("expected sequence 1, got %d", entry.Sequence)
	}
}
