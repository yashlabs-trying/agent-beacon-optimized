// Audit-only stage timer. Copy into a disposable learning package checkout.
package learning

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestProfileV7Stages(t *testing.T) {
	path := os.Getenv("BEACON_AUDIT_DB")
	if path == "" { t.Skip("audit-only: database path required") }
	s := OpenSemantic(path, NewHashedEmbedder())
	defer s.Close()
	measure := func(name string, fn func() error) {
		t.Helper()
		start := time.Now()
		if err := fn(); err != nil { t.Fatalf("%s: %v", name, err) }
		t.Logf("STAGE %s %d", name, time.Since(start).Nanoseconds())
	}
	var query Query
	var dbReady bool
	measure("db_open_schema", func() error { _, err := s.db(); dbReady = err == nil; return err })
	if !dbReady { t.Fatal("db not ready") }
	query = Query{ProjectPath: os.Getenv("BEACON_AUDIT_PROJECT_PATH"), Q: os.Getenv("BEACON_AUDIT_QUERY"), Limit: 50}
	measure("scope", func() error { var err error; query, err = s.scopeQuery(query); return err })
	measure("fts_ready", func() error { db, _ := s.db(); if !s.ensureIndexReady(db) { t.Fatal("index unavailable") }; return nil })
	measure("lexical", func() error { _, err := s.ListMemories(query); return err })
	if os.Getenv("BEACON_AUDIT_SEMANTIC") == "1" {
		measure("embedding_backfill", func() error { db, _ := s.db(); return s.ensureQuestionEmbeddings(db, query.ProjectID) })
		measure("exact_vector", func() error {
			db, _ := s.db()
			_, err := s.semanticSearchExact(db, query, s.embedder.Embed(query.Q), 50)
			return err
		})
	}
	var result QuestionResult
	measure("question_warm", func() error { var err error; result, err = s.SearchQuestion(query, os.Getenv("BEACON_AUDIT_QUESTION"), QuestionOptions{}); return err })
	measure("json_encode", func() error { _, err := json.Marshal(result); return err })
	t.Logf("ROUTE %s %d %d", result.Route, result.LexicalCandidates, result.SemanticCandidates)
}
