package store

import "testing"

func TestInMemoryStoreSequenceAndOrdering(t *testing.T) {
	t.Parallel()

	store := NewInMemoryStore()

	first, err := store.Append("user", "first")
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	second, err := store.Append("assistant", "second")
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	if first.Sequence != 1 || second.Sequence != 2 {
		t.Fatalf("expected sequence incrementing, got %d and %d", first.Sequence, second.Sequence)
	}

	entries, err := store.ListAll()
	if err != nil {
		t.Fatalf("list all failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Content != "first" || entries[1].Content != "second" {
		t.Fatalf("expected append ordering, got %#v", entries)
	}
}

func TestInMemoryStoreListSince(t *testing.T) {
	t.Parallel()

	store := NewInMemoryStore()
	for _, content := range []string{"one", "two", "three"} {
		if _, err := store.Append("test", content); err != nil {
			t.Fatalf("append failed: %v", err)
		}
	}

	entries, err := store.ListSince(2)
	if err != nil {
		t.Fatalf("list since failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Sequence != 3 {
		t.Fatalf("expected sequence 3, got %d", entries[0].Sequence)
	}
}
