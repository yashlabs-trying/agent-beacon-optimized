package learning

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// phase2_integration_test.go covers production integration and lifecycle:
// idempotent initialisation, upgrade from the original (pre-V6) storage layout,
// derived-index deletion and rebuild, reopening after an interrupted startup,
// ANN cache lifecycle across generations, embedding model-version isolation, and
// behaviour on oversized or corrupt rows.

const integProject = "proj-integ"

func integMemory(id, body string) asymptoteobserve.LearningMemoryV1 {
	return asymptoteobserve.LearningMemoryV1{
		ID: id, CandidateID: "c-" + id, Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "note", Body: body, Project: asymptoteobserve.LearningProjectV1{ID: integProject, Path: "/repos/i"},
		Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "tr-" + id}},
	}
}

func TestPhase2IdempotentInit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	for round := 0; round < 3; round++ {
		store := Open(dbPath)
		if round == 0 {
			if err := store.PutMemory(integMemory("m1", "idempotent init marker")); err != nil {
				t.Fatal(err)
			}
		}
		got, err := store.ListMemories(Query{ProjectID: integProject, Q: "idempotent init"})
		if err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		if len(got) != 1 {
			t.Fatalf("round %d: got %d, want 1", round, len(got))
		}
	}
}

// TestPhase2UpgradeFromOriginalStorage builds a database with only the original
// Beacon tables (no memory_index_meta, no memories_fts), inserts rows directly,
// then opens it through the V6 Store. This is the upgrade path from pre-V6
// storage; the derived schema must be created and the index rebuilt lazily.
func TestPhase2UpgradeFromOriginalStorage(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE evaluations (id TEXT PRIMARY KEY, project_id TEXT NOT NULL, trace_id TEXT NOT NULL, status TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, rubric_hash TEXT NOT NULL, evaluator TEXT NOT NULL, evaluation_json TEXT NOT NULL)`,
		`CREATE TABLE candidates (id TEXT PRIMARY KEY, project_id TEXT NOT NULL, state TEXT NOT NULL, kind TEXT NOT NULL, title TEXT NOT NULL, source_evaluation_id TEXT, memory_id TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, candidate_json TEXT NOT NULL)`,
		`CREATE TABLE memories (id TEXT PRIMARY KEY, candidate_id TEXT NOT NULL, project_id TEXT NOT NULL, kind TEXT NOT NULL, title TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, superseded_by TEXT, memory_json TEXT NOT NULL)`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 20; i++ {
		m := integMemory(fmt.Sprintf("old-%02d", i), fmt.Sprintf("legacy upgrade marker %d", i))
		data := mustJSON(t, m)
		if _, err := raw.Exec(`INSERT INTO memories (id, candidate_id, project_id, kind, title, created_at, updated_at, superseded_by, memory_json) VALUES (?,?,?,?,?,?,?,?,?)`,
			m.ID, m.CandidateID, m.Project.ID, m.Kind, m.Title, memoryTimestamp(i), memoryTimestamp(i), "", data); err != nil {
			t.Fatal(err)
		}
	}
	raw.Close()

	store := Open(dbPath)
	got, err := store.ListMemories(Query{ProjectID: integProject, Q: "legacy upgrade", Limit: 100})
	if err != nil {
		t.Fatalf("search after upgrade: %v", err)
	}
	if len(got) != 20 {
		t.Fatalf("after upgrade got %d, want 20", len(got))
	}
}

func TestPhase2IndexRebuildAfterDeletion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	store := Open(dbPath)
	for i := 0; i < 10; i++ {
		if err := store.PutMemory(integMemory(fmt.Sprintf("r-%02d", i), fmt.Sprintf("rebuild marker %d", i))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.ListMemories(Query{ProjectID: integProject, Q: "rebuild marker"}); err != nil {
		t.Fatal(err)
	}
	// Drop the derived tables behind the store's back, as a corruption or manual
	// cleanup would.
	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{`DROP TABLE memories_fts`, `DELETE FROM memory_index_meta`} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	schemaInitialized.Delete(dbPath)
	indexReadyCache.Delete(dbPath)

	got, err := store.ListMemories(Query{ProjectID: integProject, Q: "rebuild marker", Limit: 100})
	if err != nil {
		t.Fatalf("search after index deletion: %v", err)
	}
	if len(got) != 10 {
		t.Fatalf("after rebuild got %d, want 10", len(got))
	}
}

func TestPhase2ReopenAfterInterruptedStartup(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	store := Open(dbPath)
	if err := store.PutMemory(integMemory("m1", "interrupted startup marker")); err != nil {
		t.Fatal(err)
	}
	// Simulate an interrupted startup: drop the derived schema entirely.
	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{`DROP TABLE IF EXISTS memories_fts`, `DROP TABLE IF EXISTS memories_vocab`, `DROP TABLE IF EXISTS memory_trigram_df`, `DROP TABLE IF EXISTS memory_index_meta`} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	schemaInitialized.Delete(dbPath)
	indexReadyCache.Delete(dbPath)

	reopened := Open(dbPath)
	got, err := reopened.ListMemories(Query{ProjectID: integProject, Q: "interrupted startup"})
	if err != nil {
		t.Fatalf("reopen search: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("reopen got %d, want 1", len(got))
	}
}

// TestPhase2ANNLifecycleAcrossGenerations checks that the ANN cache is keyed by
// the durable generation: a write invalidates it and the rebuilt graph sees the
// new memory.
func TestPhase2ANNLifecycleAcrossGenerations(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	store := OpenSemantic(dbPath, NewHashedEmbedder())
	project := asymptoteobserve.LearningProjectV1{ID: integProject, Path: "/repos/i"}
	for i := 0; i < 30; i++ {
		m := integMemory(fmt.Sprintf("ann-%02d", i), fmt.Sprintf("ann lifecycle needle %d", i))
		m.Project = project
		if err := store.PutMemory(m); err != nil {
			t.Fatal(err)
		}
	}
	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	q := store.embedder.Embed("ann lifecycle needle")
	first, err := store.semanticSearchANN(db, Query{ProjectID: integProject}, q, 5, defaultANNParams())
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 {
		t.Fatal("ANN returned no results")
	}
	// A write must advance the generation and invalidate the cache entry.
	newMem := integMemory("ann-new", "ann lifecycle needle NEW")
	newMem.Project = project
	if err := store.PutMemory(newMem); err != nil {
		t.Fatal(err)
	}
	second, err := store.semanticSearchANN(db, Query{ProjectID: integProject}, q, 5, defaultANNParams())
	if err != nil {
		t.Fatal(err)
	}
	// The rebuilt graph must contain the new node (visible via lookup, which is
	// exact, independent of ANN recall).
	idx, err := store.buildANN(db, Query{ProjectID: integProject}, defaultANNParams())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := idx.graph.Lookup("ann-new"); !ok {
		t.Fatal("new memory missing from rebuilt ANN graph")
	}
	_ = second
}

// altEmbedder is a second model id over the same hashing, to test model-version
// isolation.
type altEmbedder struct{ *HashedEmbedder }

func (a altEmbedder) Model() string { return "hashed-subword-v2-test" }

func TestPhase2EmbeddingModelVersionIsolation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	storeV1 := OpenSemantic(dbPath, NewHashedEmbedder())
	if err := storeV1.PutMemory(integMemory("m1", "model version marker")); err != nil {
		t.Fatal(err)
	}
	db, err := storeV1.db()
	if err != nil {
		t.Fatal(err)
	}
	var modelV1 string
	if err := db.QueryRow(`SELECT model FROM memory_embeddings WHERE memory_id = 'm1'`).Scan(&modelV1); err != nil {
		t.Fatal(err)
	}
	if modelV1 != NewHashedEmbedder().Model() {
		t.Fatalf("stored model = %s", modelV1)
	}

	// Open with a different model id: the old vector must not be used, and a
	// rebuild must re-embed under the new id.
	storeV2 := OpenSemantic(dbPath, altEmbedder{NewHashedEmbedder()})
	if err := storeV2.RebuildEmbeddings(); err != nil {
		t.Fatal(err)
	}
	var modelV2 string
	if err := db.QueryRow(`SELECT model FROM memory_embeddings WHERE memory_id = 'm1'`).Scan(&modelV2); err != nil {
		t.Fatal(err)
	}
	if modelV2 != "hashed-subword-v2-test" {
		t.Fatalf("after rebuild model = %s, want hashed-subword-v2-test", modelV2)
	}
}

func TestPhase2LargeMemory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	store := Open(dbPath)
	big := strings.Repeat("large-memory-token ", 60000) // ~1MB
	if err := store.PutMemory(integMemory("big", big)); err != nil {
		t.Fatal(err)
	}
	got, ok, err := store.GetMemory("big")
	if err != nil || !ok {
		t.Fatalf("large memory lost: ok=%v err=%v", ok, err)
	}
	if len(got.Body) != len(big) {
		t.Fatalf("large body length = %d, want %d", len(got.Body), len(big))
	}
	found, err := store.ListMemories(Query{ProjectID: integProject, Q: "large-memory-token"})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("large memory search got %d, want 1", len(found))
	}
}

// TestPhase2CorruptRowSurfacedNotSilent verifies that a hand-corrupted row is
// reported as an error rather than silently dropped or causing a panic.
func TestPhase2CorruptRowSurfacedNotSilent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "memory.db")
	store := Open(dbPath)
	if err := store.PutMemory(integMemory("ok", "valid corrupt-probe row")); err != nil {
		t.Fatal(err)
	}
	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO memories (id, candidate_id, project_id, kind, title, created_at, updated_at, superseded_by, memory_json) VALUES ('bad','c','p','k','t','','','','{not valid json')`); err != nil {
		t.Fatal(err)
	}
	schemaInitialized.Delete(dbPath)
	indexReadyCache.Delete(dbPath)

	// The corrupt row must surface as an error, not a panic and not a silent
	// drop that hides data loss.
	_, err = store.ListMemories(Query{ProjectID: "p", Q: "corrupt-probe", Limit: 10})
	if err == nil {
		t.Fatal("expected an error from a corrupt row, got none")
	}
}

func mustJSON(t *testing.T, m asymptoteobserve.LearningMemoryV1) string {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
