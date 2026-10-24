package mcpserver

// P2 tests: default-safe project authorization. A default server (no explicit
// authorizer) is scoped to the project of its own working directory and denies
// cross-project reads, searches, and context; an explicit allowlist authorizes
// exactly its listed projects.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymptote-labs/agent-beacon/cli/beacon/internal/learning"
)

func TestP2DefaultGetMemoryCrossProjectDenied(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-secret", "memory-secret",
		"Secret convention", "Internal credential rotation procedure.")
	server := New(Options{LogPath: logPath})

	result, err := server.callTool(t.Context(), callToolParams{
		Name:      "get_memory",
		Arguments: map[string]interface{}{"id": "memory-secret"},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("default must deny cross-project get_memory: %s", result.Content[0].Text)
	}
	if strings.Contains(result.Content[0].Text, "credential rotation") {
		t.Fatalf("denial leaked memory body: %s", result.Content[0].Text)
	}
}

func TestP2DefaultSearchCrossProjectDenied(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-secret", "memory-secret",
		"Rotate the credential needle", "Body about the secret needle procedure.")
	server := New(Options{LogPath: logPath})

	result, err := server.callTool(t.Context(), callToolParams{
		Name:      "search_memory",
		Arguments: map[string]interface{}{"project_id": "project-secret", "q": "needle"},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("default must deny cross-project search: %s", result.Content[0].Text)
	}
	if strings.Contains(result.Content[0].Text, "needle procedure") {
		t.Fatalf("denial leaked memory body: %s", result.Content[0].Text)
	}
}

func TestP2DefaultContextCrossProjectDenied(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-secret", "memory-secret",
		"Rotate the credential needle", "Body about the secret needle procedure.")
	server := New(Options{LogPath: logPath})

	result, err := server.callTool(t.Context(), callToolParams{
		Name:      "get_memory_context",
		Arguments: map[string]interface{}{"project_id": "project-secret", "task": "needle"},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("default must deny cross-project context: %s", result.Content[0].Text)
	}
}

func TestP2ForgedProjectIDDenied(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-secret", "memory-secret", "t", "b")
	server := New(Options{LogPath: logPath})

	// A caller cannot gain access merely by naming another project.
	for _, args := range []map[string]interface{}{
		{"project_id": "project-secret", "q": "t"},
		{"project": "/some/other/path", "q": "t"},
	} {
		result, err := server.callTool(t.Context(), callToolParams{Name: "search_memory", Arguments: args})
		if err != nil {
			t.Fatalf("transport error: %v", err)
		}
		if !result.IsError {
			t.Fatalf("forged scope must be denied: args=%v got=%s", args, result.Content[0].Text)
		}
	}
}

func TestP2AuthorizedSameProjectWorks(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	cwd, err := learning.ResolveProject("")
	if err != nil {
		t.Skipf("cannot resolve working-directory project: %v", err)
	}
	seedMemoryForTest(t, logPath, cwd.ID, "memory-own", "Own needle", "Own body needle procedure.")
	server := New(Options{LogPath: logPath})

	// get_memory (SQL-scoped to the single authorized project)
	text := toolText(t, server, "get_memory", map[string]interface{}{"id": "memory-own"})
	if !strings.Contains(text, "Own body") {
		t.Fatalf("authorized get_memory failed: %s", text)
	}
	// search
	text = toolText(t, server, "search_memory", map[string]interface{}{"project_id": cwd.ID, "q": "needle"})
	if !strings.Contains(text, "memory-own") {
		t.Fatalf("authorized search failed: %s", text)
	}
}

func TestP2ExplicitScopeAllowsListedProject(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-a", "memory-a", "A needle", "A body needle.")
	seedMemoryForTest(t, logPath, "project-b", "memory-b", "B needle", "B body needle.")
	server := New(Options{LogPath: logPath, Authorizer: NewProjectAllowlist("project-a")})

	if text := toolText(t, server, "search_memory", map[string]interface{}{"project_id": "project-a", "q": "needle"}); !strings.Contains(text, "memory-a") {
		t.Fatalf("listed project search failed: %s", text)
	}
	result, err := server.callTool(t.Context(), callToolParams{
		Name:      "search_memory",
		Arguments: map[string]interface{}{"project_id": "project-b", "q": "needle"},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("unlisted project must be denied: %s", result.Content[0].Text)
	}
}

// TestP2AuthorizeAllOptIn documents the explicit, non-default permissive mode.
func TestP2AuthorizeAllOptIn(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-secret", "memory-secret", "Secret", "Secret body here.")
	server := New(Options{LogPath: logPath, AuthorizeAll: true})

	text := toolText(t, server, "get_memory", map[string]interface{}{"id": "memory-secret"})
	if !strings.Contains(text, "Secret body here.") {
		t.Fatalf("AuthorizeAll opt-in should return the memory: %s", text)
	}
}
