package secrets

import (
	"context"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	key1, err := DeriveKey("test-secret")
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	if len(key1) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key1))
	}

	// Deterministic
	key2, err := DeriveKey("test-secret")
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	if string(key1) != string(key2) {
		t.Fatal("HKDF not deterministic")
	}

	// Different secret produces different key
	key3, err := DeriveKey("different-secret")
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	if string(key1) == string(key3) {
		t.Fatal("different secrets produced same key")
	}

	// Empty secret fails
	_, err = DeriveKey("")
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key, _ := DeriveKey("test-secret")
	plaintext := []byte("sk-ant-test-key-12345678")

	ct, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Ciphertext should be different from plaintext
	if string(ct) == string(plaintext) {
		t.Fatal("ciphertext equals plaintext")
	}

	// Decrypt round-trip
	result, err := Decrypt(key, ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(result) != string(plaintext) {
		t.Fatalf("round-trip failed: got %q, want %q", result, plaintext)
	}

	// Wrong key fails
	wrongKey, _ := DeriveKey("wrong-secret")
	_, err = Decrypt(wrongKey, ct)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}

	// Tampered ciphertext fails
	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[len(tampered)-1] ^= 0xff
	_, err = Decrypt(key, tampered)
	if err == nil {
		t.Fatal("expected error decrypting tampered ciphertext")
	}
}

func TestMemoryStore(t *testing.T) {
	key, _ := DeriveKey("test-secret")
	store := NewMemoryStore(key)
	ctx := context.Background()

	// Put
	meta, err := store.Put(ctx, "anthropic", "sk-ant-test1234")
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if meta.Provider != "anthropic" {
		t.Fatalf("expected provider 'anthropic', got %q", meta.Provider)
	}
	if meta.Last4 != "1234" {
		t.Fatalf("expected last4 '1234', got %q", meta.Last4)
	}
	if meta.Source != "store" {
		t.Fatalf("expected source 'store', got %q", meta.Source)
	}

	// Get (internal use)
	plaintext, err := store.Get(ctx, "anthropic")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if plaintext != "sk-ant-test1234" {
		t.Fatalf("Get: got %q, want %q", plaintext, "sk-ant-test1234")
	}

	// Meta (no plaintext)
	m, err := store.Meta(ctx, "anthropic")
	if err != nil {
		t.Fatalf("Meta: %v", err)
	}
	if m.Last4 != "1234" {
		t.Fatalf("Meta.Last4: got %q, want %q", m.Last4, "1234")
	}

	// List
	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List: got %d entries, want 1", len(list))
	}

	// Add a second
	store.Put(ctx, "openai", "sk-openai-test5678")

	list, _ = store.List(ctx)
	if len(list) != 2 {
		t.Fatalf("List: got %d entries, want 2", len(list))
	}

	// Delete
	err = store.Delete(ctx, "anthropic")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = store.Get(ctx, "anthropic")
	if err == nil {
		t.Fatal("expected error after delete")
	}

	// Delete non-existent
	err = store.Delete(ctx, "anthropic")
	if err == nil {
		t.Fatal("expected error deleting non-existent")
	}

	// Overwrite
	store.Put(ctx, "openai", "sk-new-key-9999")
	plaintext, _ = store.Get(ctx, "openai")
	if plaintext != "sk-new-key-9999" {
		t.Fatalf("overwrite failed: got %q", plaintext)
	}
}

func TestLast4(t *testing.T) {
	tests := []struct{ in, want string }{
		{"sk-ant-test1234", "1234"},
		{"abc", "abc"},
		{"", ""},
		{"abcd", "abcd"},
		{"abcde", "bcde"},
	}
	for _, tt := range tests {
		if got := last4(tt.in); got != tt.want {
			t.Errorf("last4(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
