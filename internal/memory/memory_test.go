package memory

import (
	"context"
	"testing"
)

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	// Empty list
	notes, err := s.List(ctx, "user1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("expected 0 notes, got %d", len(notes))
	}

	// Add notes
	n1, err := s.Add(ctx, "user1", "I prefer Go over Rust")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if n1.Text != "I prefer Go over Rust" {
		t.Fatalf("unexpected note text: %s", n1.Text)
	}
	if n1.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	n2, err := s.Add(ctx, "user1", "My timezone is PST")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Different user
	_, err = s.Add(ctx, "user2", "I like Python")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// List for user1
	notes, err = s.List(ctx, "user1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}

	// List for user2
	notes, err = s.List(ctx, "user2")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}

	// Delete specific note
	if err := s.Delete(ctx, "user1", n1.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	notes, err = s.List(ctx, "user1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note after delete, got %d", len(notes))
	}
	if notes[0].ID != n2.ID {
		t.Fatalf("wrong note remaining: got %d, want %d", notes[0].ID, n2.ID)
	}

	// Delete wrong user's note
	if err := s.Delete(ctx, "user2", n2.ID); err == nil {
		t.Fatal("expected error deleting another user's note")
	}

	// Delete all
	count, err := s.DeleteAll(ctx, "user1")
	if err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 deleted, got %d", count)
	}
	notes, err = s.List(ctx, "user1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("expected 0 notes after delete all, got %d", len(notes))
	}
}
