package mcpserver

import (
	"strings"
	"testing"
)

func TestResourceOversizedQueryRejected(t *testing.T) {
	server := New(Options{LogPath: "runtime.jsonl"})
	result, err := server.callTool(t.Context(), callToolParams{
		Name:      "search_memory",
		Arguments: map[string]interface{}{"q": strings.Repeat("a", maxMemoryQueryBytes+1)},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError || !strings.Contains(result.Content[0].Text, "too long") {
		t.Fatalf("expected an explicit too-long rejection, got: %s", result.Content[0].Text)
	}
}

func TestResourceTooManyTermsRejected(t *testing.T) {
	server := New(Options{LogPath: "runtime.jsonl"})
	terms := make([]string, maxMemoryQueryTerms+1)
	for i := range terms {
		terms[i] = "aa"
	}
	result, err := server.callTool(t.Context(), callToolParams{
		Name:      "search_memory",
		Arguments: map[string]interface{}{"q": strings.Join(terms, " ")},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError || !strings.Contains(result.Content[0].Text, "too many terms") {
		t.Fatalf("expected a too-many-terms rejection, got: %s", result.Content[0].Text)
	}
}

func TestResourceOversizedIDRejected(t *testing.T) {
	server := New(Options{LogPath: "runtime.jsonl"})
	result, err := server.callTool(t.Context(), callToolParams{
		Name:      "get_memory",
		Arguments: map[string]interface{}{"id": strings.Repeat("x", maxMemoryIDBytes+1)},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError || !strings.Contains(result.Content[0].Text, "too long") {
		t.Fatalf("expected an explicit too-long rejection, got: %s", result.Content[0].Text)
	}
}
