// Audit-only benchmark. Copy into cli/beacon/internal/learning in a disposable
// checkout; it is not part of the production package or release test suite.
package learning

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func profileMemory(id, title, body string) asymptoteobserve.LearningMemoryV1 {
	return asymptoteobserve.LearningMemoryV1{
		ID: id, CandidateID: "candidate-" + id,
		Project: asymptoteobserve.LearningProjectV1{ID: "p"},
		Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: title, Body: body, Tags: []string{"source:verified"},
		Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: "trace-" + id}},
	}
}

func profileStore(b *testing.B, n int) *Store {
	b.Helper()
	s := Open(filepath.Join(b.TempDir(), "memory.db"))
	for i := 0; i < n-1; i++ {
		m := profileMemory(fmt.Sprintf("noise-%04d", i), "Inventory record", fmt.Sprintf("Record %d uses inert stock code %d.", i, i))
		if err := s.PutMemory(m); err != nil { b.Fatal(err) }
	}
	if err := s.PutMemory(profileMemory("fact", "Orion environment", "The orion service deploys to staging.")); err != nil { b.Fatal(err) }
	return s
}

func BenchmarkProfileV7Direct(b *testing.B) {
	s := profileStore(b, 1000)
	defer s.Close()
	q := Query{ProjectID: "p", Q: "orion service", Limit: 50}
	question := "Which environment does the orion service deploy to?"
	if _, err := s.SearchQuestion(q, question, QuestionOptions{}); err != nil { b.Fatal(err) }
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.SearchQuestion(q, question, QuestionOptions{}); err != nil { b.Fatal(err) }
	}
}

func BenchmarkProfileV7List(b *testing.B) {
	s := profileStore(b, 1000)
	defer s.Close()
	q := Query{ProjectID: "p", Q: "orion service", Limit: 50}
	if _, err := s.ListMemories(q); err != nil { b.Fatal(err) }
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.ListMemories(q); err != nil { b.Fatal(err) }
	}
}

func BenchmarkProfileV7SemanticWarm(b *testing.B) {
	s := Open(filepath.Join(b.TempDir(), "memory.db"))
	for i := 0; i < 199; i++ {
		if err := s.PutMemory(profileMemory(fmt.Sprintf("noise-%03d", i), "Inventory record", fmt.Sprintf("Record %d uses inert stock code %d.", i, i))); err != nil { b.Fatal(err) }
	}
	if err := s.PutMemory(profileMemory("bridge", "Archive location", "The archive service stores data in the atlas database.")); err != nil { b.Fatal(err) }
	if err := s.Close(); err != nil { b.Fatal(err) }
	s = OpenSemantic(s.Path(), NewHashedEmbedder())
	defer s.Close()
	q := Query{ProjectID: "p", Q: "archive service"}
	question := "Which port does the archive service database listen on?"
	if _, err := s.SearchQuestion(q, question, QuestionOptions{}); err != nil { b.Fatal(err) }
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.SearchQuestion(q, question, QuestionOptions{}); err != nil { b.Fatal(err) }
	}
}

func TestProfileV7SemanticFirst(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "memory.db"))
	for i := 0; i < 200; i++ {
		if err := s.PutMemory(profileMemory(fmt.Sprintf("noise-%03d", i), "Inventory record", fmt.Sprintf("Record %d uses inert stock code %d.", i, i))); err != nil { t.Fatal(err) }
	}
	if err := s.PutMemory(profileMemory("bridge", "Archive location", "The archive service stores data in the atlas database.")); err != nil { t.Fatal(err) }
	if err := s.Close(); err != nil { t.Fatal(err) }
	s = OpenSemantic(s.Path(), NewHashedEmbedder())
	defer s.Close()
	q := Query{ProjectID: "p", Q: "archive service"}
	question := "Which port does the archive service database listen on?"
	start := time.Now()
	first, err := s.SearchQuestion(q, question, QuestionOptions{})
	if err != nil { t.Fatal(err) }
	firstElapsed := time.Since(start)
	start = time.Now()
	second, err := s.SearchQuestion(q, question, QuestionOptions{})
	if err != nil { t.Fatal(err) }
	t.Logf("first=%s second=%s route=%s/%s candidates=%d/%d", firstElapsed, time.Since(start), first.Route, second.Route, first.SemanticCandidates, second.SemanticCandidates)
}
