package errors

import (
	"fmt"
	"strings"
	"testing"
)

func TestError_Format(t *testing.T) {
	err := New(CodeProviderAuth, "API key is invalid")
	got := err.Error()
	if !strings.Contains(got, "[PROVIDER_AUTH]") {
		t.Fatalf("expected code in error string, got: %s", got)
	}
	if !strings.Contains(got, "API key is invalid") {
		t.Fatalf("expected message in error string, got: %s", got)
	}
}

func TestError_Wrap(t *testing.T) {
	cause := fmt.Errorf("connection refused")
	err := Wrap(CodeStateConnect, "database connection failed", cause)
	got := err.Error()
	if !strings.Contains(got, "connection refused") {
		t.Fatalf("expected cause in error string, got: %s", got)
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("underlying")
	err := Wrap(CodeStateConnect, "wrapper", cause)
	if err.Unwrap() != cause {
		t.Fatal("Unwrap should return cause")
	}
}

func TestError_WithFix(t *testing.T) {
	err := New(CodeConfigMissing, "config not found").
		WithFix("Run tide-cli init").
		WithDocs("https://example.com/docs")
	if err.Fix != "Run tide-cli init" {
		t.Fatalf("expected fix, got: %s", err.Fix)
	}
	if err.DocsURL != "https://example.com/docs" {
		t.Fatalf("expected docs URL, got: %s", err.DocsURL)
	}
}
