package learning

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// Phase C warm-path benchmark: question-aware search versus the original
// ListMemories path on an identical warm 1,000-memory store.
func phaseCStore(b *testing.B, n int) *Store {
	b.Helper()
	s := Open(filepath.Join(b.TempDir(), "memory.db"))
	for i := 0; i < n; i++ {
		g := i % 50
		m := asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("m%d", i), CandidateID: fmt.Sprintf("c%d", i),
			Kind:     asymptoteobserve.LearningMemoryKindConvention,
			Title:    fmt.Sprintf("note %d", i),
			Body:     fmt.Sprintf("The component%d setting value is %d.", g, i),
			Project:  asymptoteobserve.LearningProjectV1{ID: "p"},
			Tags:     []string{"source:verified"},
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: fmt.Sprintf("t%d", i)}},
		}
		if err := s.PutMemory(m); err != nil {
			b.Fatal(err)
		}
	}
	// Warm the index and schema so we measure steady state, not first use.
	if _, err := s.ListMemories(Query{ProjectID: "p", Q: "component0 setting", Limit: 8}); err != nil {
		b.Fatal(err)
	}
	return s
}

func BenchmarkPhaseCListMemoriesWarm(b *testing.B) {
	s := phaseCStore(b, 1000)
	defer s.Close()
	q := Query{ProjectID: "p", Q: "component0 setting value", Limit: 8}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.ListMemories(q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhaseCSearchQuestionWarm(b *testing.B) {
	s := phaseCStore(b, 1000)
	defer s.Close()
	q := Query{ProjectID: "p", Q: "component0 setting value"}
	question := "What is the component0 setting value?"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.SearchQuestion(q, question, QuestionOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPhaseCOpenColdExisting measures opening an already-populated store and
// running one lexical query — the cold process path for a healthy database.
func BenchmarkPhaseCOpenColdExisting(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "memory.db")
	seed := Open(path)
	for i := 0; i < 1000; i++ {
		g := i % 50
		m := asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("m%d", i), CandidateID: fmt.Sprintf("c%d", i),
			Kind: asymptoteobserve.LearningMemoryKindConvention, Title: fmt.Sprintf("note %d", i),
			Body:    fmt.Sprintf("The component%d setting value is %d.", g, i),
			Project: asymptoteobserve.LearningProjectV1{ID: "p"}, Tags: []string{"source:verified"},
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: fmt.Sprintf("t%d", i)}},
		}
		if err := seed.PutMemory(m); err != nil {
			b.Fatal(err)
		}
	}
	if _, err := seed.ListMemories(Query{ProjectID: "p", Q: "component0 setting", Limit: 8}); err != nil {
		b.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := Open(path)
		if _, err := s.ListMemories(Query{ProjectID: "p", Q: "component0 setting", Limit: 8}); err != nil {
			b.Fatal(err)
		}
		_ = s.Close()
	}
}
