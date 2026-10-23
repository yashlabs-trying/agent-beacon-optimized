package learning

// Phase 3A Workstream A: retrieval-quality harness over the locked
// reports/phase3/heldout/retrieval_context_v1.json fixture.
//
// This file is deliberately written against only the baseline-stable API
// (Open, PutMemory, ListMemories, Query) so the exact same source compiles and
// runs on the pre-V6 baseline (99c70908) and on the Phase 3A worktree. The
// lexical arm is therefore directly comparable across both codebases.
//
// It is opt-in: set BEACON_PHASE3_RETRIEVAL=1 to run. The fixture path defaults
// to <repo>/reports/phase3/heldout/retrieval_context_v1.json and can be
// overridden with BEACON_PHASE3_RETRIEVAL_FIXTURE (used to point the baseline
// run at the locked fixture without copying it).

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

type p3Queries struct {
	Identifier   string `json:"identifier"`
	Paraphrase   string `json:"paraphrase"`
	Synonym      string `json:"synonym"`
	HardNegative string `json:"hard_negative"`
}

type p3Topic struct {
	ID        string    `json:"id"`
	Primary   string    `json:"primary"`
	Support   string    `json:"support"`
	Duplicate string    `json:"duplicate"`
	Decoy     string    `json:"decoy"`
	Queries   p3Queries `json:"queries"`
}

type p3RetrievalFixture struct {
	SchemaVersion int       `json:"schema_version"`
	DatasetID     string    `json:"dataset_id"`
	ProjectID     string    `json:"project_id"`
	Topics        []p3Topic `json:"topics"`
}

func p3RetrievalFixturePath(t *testing.T) string {
	if p := os.Getenv("BEACON_PHASE3_RETRIEVAL_FIXTURE"); p != "" {
		return p
	}
	return filepath.Join("..", "..", "..", "..", "reports", "phase3", "heldout", "retrieval_context_v1.json")
}

func p3LoadRetrievalFixture(t *testing.T) p3RetrievalFixture {
	t.Helper()
	path := p3RetrievalFixturePath(t)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("locked retrieval fixture not found at %s: %v", path, err)
	}
	var fx p3RetrievalFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if len(fx.Topics) != 12 {
		t.Fatalf("expected 12 topics, got %d", len(fx.Topics))
	}
	return fx
}

func p3Memory(id, project, kind, title, body, traceID string) asymptoteobserve.LearningMemoryV1 {
	return asymptoteobserve.LearningMemoryV1{
		ID:          id,
		CandidateID: "cand-" + id,
		Kind:        kind,
		Title:       title,
		Body:        body,
		Project:     asymptoteobserve.LearningProjectV1{ID: project},
		Evidence:    []asymptoteobserve.LearningEvidenceV1{{TraceID: traceID}},
	}
}

// p3SeedRetrieval inserts the 12 topics x 4 memories into a freshly opened
// lexical store and returns the fixture. Order is newest-last so that ties in
// the baseline (ORDER BY updated_at DESC) are deterministic.
func p3SeedRetrieval(t *testing.T, fx p3RetrievalFixture) *Store {
	t.Helper()
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	for _, topic := range fx.Topics {
		mems := []asymptoteobserve.LearningMemoryV1{
			p3Memory(topic.ID+"-primary", fx.ProjectID, "convention", topic.ID+" primary", topic.Primary, topic.ID+"-primary"),
			p3Memory(topic.ID+"-duplicate", fx.ProjectID, "convention", topic.ID+" duplicate", topic.Duplicate, topic.ID+"-primary"),
			p3Memory(topic.ID+"-support", fx.ProjectID, "gotcha", topic.ID+" support", topic.Support, topic.ID+"-support"),
			p3Memory(topic.ID+"-decoy", fx.ProjectID, "workflow", topic.ID+" decoy", topic.Decoy, topic.ID+"-decoy"),
		}
		for _, m := range mems {
			if err := store.PutMemory(m); err != nil {
				t.Fatalf("put %s: %v", m.ID, err)
			}
		}
	}
	return store
}

// p3QueryCase is one of the 48 locked queries with its category and gold set.
type p3QueryCase struct {
	ID       string
	Category string
	Text     string
	Gold     []string
}

func p3QueryCases(fx p3RetrievalFixture) []p3QueryCase {
	var out []p3QueryCase
	for _, topic := range fx.Topics {
		gold := []string{topic.ID + "-primary", topic.ID + "-duplicate"}
		out = append(out,
			p3QueryCase{ID: topic.ID + "-identifier", Category: "identifier", Text: topic.Queries.Identifier, Gold: gold},
			p3QueryCase{ID: topic.ID + "-paraphrase", Category: "paraphrase", Text: topic.Queries.Paraphrase, Gold: gold},
			p3QueryCase{ID: topic.ID + "-synonym", Category: "synonym", Text: topic.Queries.Synonym, Gold: gold},
			p3QueryCase{ID: topic.ID + "-hard_negative", Category: "hard_negative", Text: topic.Queries.HardNegative, Gold: gold},
		)
	}
	return out
}

type p3Metrics struct {
	n               int
	r1, r5, mrr, nd float64
	lat             []time.Duration
}

func (m *p3Metrics) add(ids []string, gold []string, d time.Duration) {
	m.n++
	m.lat = append(m.lat, d)
	goldSet := map[string]bool{}
	for _, g := range gold {
		goldSet[g] = true
	}
	hit1, hit5, rr, dcg := 0.0, 0.0, 0.0, 0.0
	seen := 0
	for i, id := range ids {
		rank := i + 1
		if goldSet[id] {
			if rank == 1 {
				hit1 = 1
			}
			if rank <= 5 {
				hit5 = 1
				dcg += 1 / math.Log2(float64(rank)+1)
			}
			if seen == 0 {
				rr = 1 / float64(rank)
			}
			seen++
		}
	}
	idcg := 0.0
	for i := 0; i < len(gold) && i < 5; i++ {
		idcg += 1 / math.Log2(float64(i+2))
	}
	nd := 0.0
	if idcg > 0 {
		nd = dcg / idcg
	}
	m.r1 += hit1
	m.r5 += hit5
	m.mrr += rr
	m.nd += nd
}

func p3Percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(len(sorted))*p)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func (m *p3Metrics) report(method, category string) {
	lat := append([]time.Duration(nil), m.lat...)
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	if m.n == 0 {
		fmt.Printf("P3A_RETRIEVAL method=%s category=%s n=0\n", method, category)
		return
	}
	fmt.Printf("P3A_RETRIEVAL method=%s category=%s n=%d R@1=%.4f R@5=%.4f MRR=%.4f NDCG@5=%.4f p50_us=%d p95_us=%d\n",
		method, category, m.n, m.r1/float64(m.n), m.r5/float64(m.n), m.mrr/float64(m.n), m.nd/float64(m.n),
		p3Percentile(lat, 0.50).Microseconds(), p3Percentile(lat, 0.95).Microseconds())
}

// p3RunMethod evaluates a method across all 48 queries and prints per-category
// and aggregate lines. run returns the ranked memory IDs (top 5) for a query.
func p3RunMethod(name string, cases []p3QueryCase, run func(c p3QueryCase) []string) {
	agg := &p3Metrics{}
	byCat := map[string]*p3Metrics{}
	for _, c := range cases {
		if byCat[c.Category] == nil {
			byCat[c.Category] = &p3Metrics{}
		}
		start := time.Now()
		ids := run(c)
		d := time.Since(start)
		agg.add(ids, c.Gold, d)
		byCat[c.Category].add(ids, c.Gold, d)
	}
	agg.report(name, "ALL")
	for _, cat := range []string{"identifier", "paraphrase", "synonym", "hard_negative"} {
		if byCat[cat] != nil {
			byCat[cat].report(name, cat)
		}
	}
}

func TestPhase3ARetrievalLexical(t *testing.T) {
	if os.Getenv("BEACON_PHASE3_RETRIEVAL") != "1" {
		t.Skip("set BEACON_PHASE3_RETRIEVAL=1 to run the locked retrieval evaluation")
	}
	fx := p3LoadRetrievalFixture(t)
	store := p3SeedRetrieval(t, fx)
	cases := p3QueryCases(fx)
	fmt.Printf("P3A_RETRIEVAL_FIXTURE dataset=%s topics=%d queries=%d\n", fx.DatasetID, len(fx.Topics), len(cases))

	p3RunMethod("lexical", cases, func(c p3QueryCase) []string {
		got, err := store.ListMemories(Query{ProjectID: fx.ProjectID, Q: c.Text, Limit: 5})
		if err != nil {
			t.Fatalf("lexical search: %v", err)
		}
		ids := make([]string, 0, len(got))
		for _, m := range got {
			ids = append(ids, m.ID)
		}
		return ids
	})
}
