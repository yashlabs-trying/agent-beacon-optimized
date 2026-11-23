package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymptote-labs/agent-beacon/cli/beacon/internal/learning"
	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// These tests reproduce the frozen-original behavior of Finding A: the local MCP
// memory surface derives its project scope entirely from caller-supplied MCP
// arguments, and get_memory performs an unscoped lookup by ID. They use only the
// pre-hardening API so they can be run against the frozen baseline commit
// 99c70908 and the V7 baseline 7c17c5fd to capture "before" evidence. The
// enforced replacements live in security_authorization_test.go.

// scratchDir returns a temp directory whose cleanup ignores failures. The MCP
// server opens a fresh, never-closed Store per call (see Server.memoryStore), so
// on Windows the SQLite file stays locked and t.TempDir's strict cleanup would
// fail the test for an unrelated reason. Cleanup is best-effort here so the
// security assertion is the only pass/fail signal.
func scratchDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "beacon-sec-repro-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func seedMemoryForTest(t *testing.T, logPath, projectID, id, title, body string) {
	t.Helper()
	store := learning.Open(learning.PathForRuntimeLog(logPath))
	if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
		ID:          id,
		CandidateID: "candidate-" + id,
		Kind:        asymptoteobserve.LearningMemoryKindConvention,
		Title:       title,
		Body:        body,
		Project:     asymptoteobserve.LearningProjectV1{ID: projectID},
	}); err != nil {
		t.Fatalf("seed memory: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close seed store: %v", err)
	}
}

func toolText(t *testing.T, server *Server, name string, args map[string]interface{}) string {
	t.Helper()
	result, err := server.callTool(t.Context(), callToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s returned error: %v", name, err)
	}
	if len(result.Content) == 0 {
		t.Fatalf("%s returned no content", name)
	}
	return result.Content[0].Text
}

// TestBaselineGetMemoryLeaksAcrossProjects documents that an unscoped
// get_memory returns a memory belonging to a project the caller never named.
func TestBaselineGetMemoryLeaksAcrossProjects(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-secret", "memory-secret",
		"Secret project convention", "Internal credential rotation procedure.")
	server := New(Options{LogPath: logPath, AuthorizeAll: true})

	text := toolText(t, server, "get_memory", map[string]interface{}{"id": "memory-secret"})
	if !strings.Contains(text, "credential rotation") {
		t.Fatalf("expected the unscoped lookup to expose the memory body, got: %s", text)
	}
	t.Logf("PRE-P2 PERMISSIVE MODE: get_memory returned another project's memory: %s", text)
}

// TestBaselineSearchTrustsCallerProjectID documents that search_memory accepts
// an arbitrary project_id and returns that project's rows.
func TestBaselineSearchTrustsCallerProjectID(t *testing.T) {
	logPath := filepath.Join(scratchDir(t), "endpoint", "logs", "runtime.jsonl")
	seedMemoryForTest(t, logPath, "project-secret", "memory-secret",
		"Rotate the credential needle", "Body text about the secret needle procedure.")
	server := New(Options{LogPath: logPath, AuthorizeAll: true})

	text := toolText(t, server, "search_memory", map[string]interface{}{
		"project_id": "project-secret",
		"q":          "needle",
	})
	if !strings.Contains(text, "needle") {
		t.Fatalf("expected caller-supplied project scope to return the memory, got: %s", text)
	}
	t.Logf("PRE-P2 PERMISSIVE MODE: search_memory honored an arbitrary caller project_id: %s", text)
}
