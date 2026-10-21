package learning

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// TestSearchDifferentialRandomized drives the adaptive path and the original
// scan with the same randomly generated datasets and queries and requires
// identical results. It covers FTS5 trigram edges: one-, two- and three-rune
// terms, Unicode, punctuation, FTS operators, repeated and mixed-case terms,
// empty and whitespace queries, pagination, limits, kind filters and projects.
func TestSearchDifferentialRandomized(t *testing.T) {
	const (
		datasetSize = 800
		queryCount  = 10000
	)
	rng := rand.New(rand.NewSource(20261021))

	vocab := []string{
		"needle", "package", "smoke", "retry", "café", "日本", "emoji😀",
		"foo-bar", "foo.bar", "a", "ab", "abc", "the", "workflow", "undo",
		"x1", "C++", "a_b", "50%", "it's", `back\slash`, `quote"inside`,
		"star*", "paren(", "colon:", "caret^", "tilde~", "bracket[", "pipe|",
		"Ünïcode", "mixedCase", "  spaced  ", "tab\there",
	}
	projects := []string{"proj-a", "proj-a", "proj-b"}
	kinds := []string{
		asymptoteobserve.LearningMemoryKindConvention,
		asymptoteobserve.LearningMemoryKindDebuggingPattern,
	}

	memories := make([]asymptoteobserve.LearningMemoryV1, 0, datasetSize)
	for i := 0; i < datasetSize; i++ {
		var body strings.Builder
		terms := 1 + rng.Intn(6)
		for j := 0; j < terms; j++ {
			if j > 0 {
				body.WriteString(" ")
			}
			body.WriteString(vocab[rng.Intn(len(vocab))])
		}
		m := asymptoteobserve.LearningMemoryV1{
			ID:          fmt.Sprintf("m-%04d", i),
			CandidateID: fmt.Sprintf("c-%04d", i),
			Kind:        kinds[rng.Intn(len(kinds))],
			Title:       fmt.Sprintf("Title %d %s", i, vocab[rng.Intn(len(vocab))]),
			Body:        body.String(),
			Project:     asymptoteobserve.LearningProjectV1{ID: projects[rng.Intn(len(projects))]},
			CreatedAt:   memoryTimestamp(i),
			UpdatedAt:   memoryTimestamp(i),
		}
		if rng.Intn(20) == 0 {
			m.SupersededBy = "m-0000"
		}
		memories = append(memories, m)
	}

	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	seedMemories(t, store, memories)

	randomTerm := func() string {
		switch rng.Intn(6) {
		case 0:
			word := vocab[rng.Intn(len(vocab))]
			if len(word) == 0 {
				return word
			}
			start := rng.Intn(len(word))
			end := start + 1 + rng.Intn(len(word)-start)
			return word[start:end]
		case 1:
			return strings.ToUpper(vocab[rng.Intn(len(vocab))])
		case 2:
			return vocab[rng.Intn(len(vocab))]
		case 3:
			return string(rune('a' + rng.Intn(26)))
		case 4:
			return "zzz" + string(rune('a'+rng.Intn(3)))
		default:
			return "needle"
		}
	}

	for iter := 0; iter < queryCount; iter++ {
		termCount := rng.Intn(5)
		parts := make([]string, 0, termCount)
		for j := 0; j < termCount; j++ {
			parts = append(parts, randomTerm())
		}
		q := strings.Join(parts, " ")
		switch rng.Intn(10) {
		case 0:
			q = "  " + q + "  "
		case 1:
			q = ""
		case 2:
			q = "   "
		case 3:
			q = q + "\t"
		}
		query := Query{
			ProjectID: projects[rng.Intn(len(projects))],
			Q:         q,
			Limit:     1 + rng.Intn(25),
			Page:      1 + rng.Intn(3),
		}
		if rng.Intn(3) == 0 {
			query.Kind = kinds[rng.Intn(len(kinds))]
		}

		want := oracleScan(t, store, query)
		got, err := store.ListMemories(query)
		if err != nil {
			t.Fatalf("iter %d query %+v: %v", iter, query, err)
		}
		if !resultsEquivalent(got, want) {
			t.Fatalf("iter %d query %+v: got %v want %v", iter, query, ids(got), ids(want))
		}
	}
}

// TestSearchDifferentialEdgeCases pins the specific tricky inputs.
func TestSearchDifferentialEdgeCases(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}
	bodies := []string{
		`quote "inside" text`,
		`star* glob and paren( here`,
		`a_b and 50% and it's`,
		`café and 日本 and emoji😀`,
		`back\slash and pipe| and caret^`,
		`mixedCase and MIXEDCASE`,
		`ab abc abcd`,
	}
	for i, body := range bodies {
		if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("m-%d", i), CandidateID: fmt.Sprintf("c-%d", i),
			Kind:  asymptoteobserve.LearningMemoryKindConvention,
			Title: fmt.Sprintf("title %d", i), Body: body, Project: project,
		}); err != nil {
			t.Fatal(err)
		}
	}
	queries := []string{
		`"inside"`, `star*`, `paren(`, `a_b`, `50%`, `it's`, `café`, `日本`,
		`emoji😀`, `back\slash`, `pipe|`, `caret^`, `mixedcase`, `MIXEDCASE`,
		`ab`, `abc`, `abcd`, `a`, `ab c`, `abc d`, `  `, ``, `"`, `*`, `(`,
		`the`, `needle`,
	}
	for _, q := range queries {
		query := Query{ProjectID: "proj-a", Q: q, Limit: 50}
		want := oracleScan(t, store, query)
		got, err := store.ListMemories(query)
		if err != nil {
			t.Fatalf("q=%q: %v", q, err)
		}
		if !resultsEquivalent(got, want) {
			t.Fatalf("q=%q: got %v want %v", q, ids(got), ids(want))
		}
	}
}
