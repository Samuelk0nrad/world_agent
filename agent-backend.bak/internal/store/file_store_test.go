package store

import (
	"path/filepath"
	"testing"
)

func TestFileStoreAppendAndListAll(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "memory.jsonl")
	store := NewFileStore(path)

	first, err := store.Append("test", "hello world")
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	second, err := store.Append("test", "hello again")
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	if first.Sequence != 1 || second.Sequence != 2 {
		t.Fatalf("expected sequences 1,2 got %d,%d", first.Sequence, second.Sequence)
	}

	entries, err := store.ListAll()
	if err != nil {
		t.Fatalf("list all failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Content != "hello world" || entries[1].Content != "hello again" {
		t.Fatalf("unexpected ordering: %#v", entries)
	}
	if entries[0].Sequence != 1 || entries[1].Sequence != 2 {
		t.Fatalf("unexpected sequences: %d,%d", entries[0].Sequence, entries[1].Sequence)
	}
}

func TestFileStoreListSince(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "memory.jsonl")
	store := NewFileStore(path)

	if _, err := store.Append("test", "one"); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if _, err := store.Append("test", "two"); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if _, err := store.Append("test", "three"); err != nil {
		t.Fatalf("append failed: %v", err)
	}

	entries, err := store.ListSince(1)
	if err != nil {
		t.Fatalf("list since failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Sequence != 2 || entries[1].Sequence != 3 {
		t.Fatalf("unexpected sequences: %d,%d", entries[0].Sequence, entries[1].Sequence)
	}
}
