package mcpserver

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymptote-labs/agent-beacon/cli/beacon/internal/learning"
)

// These tests exercise the Phase 2 authorization boundary: every memory tool is
// gated by the server-side Authorizer, and a caller cannot substitute a project
// it is not permitted to reach. They are the enforced counterparts to the
// baseline reproductions in security_baseline_repro_test.go.

func allowlistServer(t *testing.T, logPath string, ids ...string) *Server {
	t.Helper()
	return New(Options{LogPath: logPath, Authorizer: NewProjectAllowlist(ids...)})
}

func TestAuthorizationGetMemoryCrossProjectDenied(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-secret", "memory-secret",
		"Secret project convention", "Internal credential rotation procedure.")
	server := allowlistServer(t, logPath, "project-a")

	result, err := server.callTool(t.Context(), callToolParams{
		Name:      "get_memory",
		Arguments: map[string]interface{}{"id": "memory-secret"},
	})
	if err != nil {
		t.Fatalf("get_memory returned a transport error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected denial, got success: %s", result.Content[0].Text)
	}
	if strings.Contains(result.Content[0].Text, "credential rotation") {
		t.Fatalf("denial leaked memory content: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "memory not found") {
		t.Fatalf("expected a non-disclosing not-found error, got: %s", result.Content[0].Text)
	}
}

func TestAuthorizationGetMemoryAllowedProjectSucceeds(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-a", "memory-a",
		"Project A convention", "Use the project A workflow.")
	server := allowlistServer(t, logPath, "project-a")

	text := toolText(t, server, "get_memory", map[string]interface{}{"id": "memory-a"})
	if !strings.Contains(text, "memory-a") {
		t.Fatalf("authorized get_memory did not return the memory: %s", text)
	}
}

func TestAuthorizationSearchRejectsUnlistedProject(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-secret", "memory-secret",
		"Rotate the credential needle", "Body about the secret needle procedure.")
	server := allowlistServer(t, logPath, "project-a")

	result, err := server.callTool(t.Context(), callToolParams{
		Name:      "search_memory",
		Arguments: map[string]interface{}{"project_id": "project-secret", "q": "needle"},
	})
	if err != nil {
		t.Fatalf("search_memory transport error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected project substitution to be denied, got: %s", result.Content[0].Text)
	}
	if strings.Contains(result.Content[0].Text, "memory-secret") {
		t.Fatalf("denial leaked another project's memory: %s", result.Content[0].Text)
	}
}

func TestAuthorizationContextRejectsUnlistedProject(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-secret", "memory-secret",
		"Rotate the credential needle", "Body about the secret needle procedure.")
	server := allowlistServer(t, logPath, "project-a")

	result, err := server.callTool(t.Context(), callToolParams{
		Name:      "get_memory_context",
		Arguments: map[string]interface{}{"project_id": "project-secret", "task": "needle"},
	})
	if err != nil {
		t.Fatalf("get_memory_context transport error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected project substitution to be denied, got: %s", result.Content[0].Text)
	}
}

func TestAuthorizationSearchAllowedProjectSucceeds(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-a", "memory-a",
		"Rotate the credential needle", "Body about the project A needle procedure.")
	server := allowlistServer(t, logPath, "project-a")

	text := toolText(t, server, "search_memory", map[string]interface{}{
		"project_id": "project-a",
		"q":          "needle",
	})
	if !strings.Contains(text, "memory-a") {
		t.Fatalf("authorized search did not return the memory: %s", text)
	}
}

func TestAuthorizationEmptyAllowlistDeniesEverything(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-a", "memory-a",
		"Project A convention", "Use the project A workflow.")
	server := allowlistServer(t, logPath) // no IDs: fail-closed

	result, err := server.callTool(t.Context(), callToolParams{
		Name:      "get_memory",
		Arguments: map[string]interface{}{"id": "memory-a"},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("empty allowlist must deny, got: %s", result.Content[0].Text)
	}
}

func TestAuthorizationUnknownProjectIDDenied(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	server := allowlistServer(t, logPath, "project-a")

	result, err := server.callTool(t.Context(), callToolParams{
		Name:      "search_memory",
		Arguments: map[string]interface{}{"project_id": "project-unknown", "q": "x"},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("unknown project must be denied, got: %s", result.Content[0].Text)
	}
}

// TestAuthorizationDefaultScopesToWorkingDirectory confirms the default server
// is scoped to the project of its own working directory: a memory in a different
// project is denied, and a memory in the working-directory project is returned.
func TestAuthorizationDefaultScopesToWorkingDirectory(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-a", "memory-a",
		"Project A convention", "Use the project A workflow.")
	server := New(Options{LogPath: logPath}) // default: CWD project scope

	// A foreign project's memory is denied (default is not allow-all).
	result, err := server.callTool(t.Context(), callToolParams{
		Name:      "get_memory",
		Arguments: map[string]interface{}{"id": "memory-a"},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("default mode must deny a foreign project's memory, got: %s", result.Content[0].Text)
	}

	// A memory in the server's own project is returned.
	cwd, err := learning.ResolveProject("")
	if err != nil {
		t.Skipf("cannot resolve working-directory project: %v", err)
	}
	seedMemoryForTest(t, logPath, cwd.ID, "memory-own", "Own convention", "Own body.")
	text := toolText(t, server, "get_memory", map[string]interface{}{"id": "memory-own"})
	if !strings.Contains(text, "Own body.") {
		t.Fatalf("default mode should return the working-directory project's memory: %s", text)
	}
}

// TestProjectAllowlistFailClosed pins the authorizer contract directly.
func TestProjectAllowlistFailClosed(t *testing.T) {
	var nilAllow *ProjectAllowlist
	if err := nilAllow.AuthorizeProject(t.Context(), "project-a"); err == nil {
		t.Fatal("nil allowlist must deny")
	}
	empty := NewProjectAllowlist()
	if err := empty.AuthorizeProject(t.Context(), "project-a"); err == nil {
		t.Fatal("empty allowlist must deny")
	}
	ok := NewProjectAllowlist("project-a")
	if err := ok.AuthorizeProject(t.Context(), "project-a"); err != nil {
		t.Fatalf("listed project must be allowed: %v", err)
	}
	if err := ok.AuthorizeProject(t.Context(), "project-b"); err == nil {
		t.Fatal("unlisted project must be denied")
	}
	if err := ok.AuthorizeProject(t.Context(), "  project-a  "); err != nil {
		t.Fatalf("whitespace should be trimmed: %v", err)
	}
}
