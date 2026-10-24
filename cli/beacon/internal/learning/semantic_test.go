package learning

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func TestHashedEmbedderBasics(t *testing.T) {
	e := NewHashedEmbedder()
	v1 := e.Embed("Authentication token expiry causes 401 responses")
	v2 := e.Embed("Authentication token expiry causes 401 responses")
	if cosine(v1, v2) < 0.999 {
		t.Fatalf("embedder not deterministic: cosine=%v", cosine(v1, v2))
	}
	if len(v1) != e.Dim() {
		t.Fatalf("dim = %d, want %d", len(v1), e.Dim())
	}
	var norm float32
	for _, x := range v1 {
		norm += x * x
	}
	if math.Abs(float64(norm)-1) > 1e-4 {
		t.Fatalf("vector not normalized: norm=%v", norm)
	}
	// Morphological overlap should score higher than unrelated text.
	morph := e.Embed("token expired refresh authentication")
	unrelated := e.Embed("kubernetes pod autoscaling memory limits")
	if cosine(v1, morph) <= cosine(v1, unrelated) {
		t.Fatalf("morphological similarity not ranked above unrelated: morph=%v unrelated=%v",
			cosine(v1, morph), cosine(v1, unrelated))
	}
}

// semCase is one labeled query. category is used only for reporting.
type semCase struct {
	category string
	query    string
	gold     string // memory body the query should retrieve
}

func semanticCases() []semCase {
	return []semCase{
		{"overlap", "stale CI artifacts after a lockfile update", "The CI cache must be invalidated after a dependency lockfile change, otherwise stale artifacts break the build."},
		{"overlap", "authentication token expired returns 401", "Authentication token expiry causes 401 responses; refresh the token before issuing the request."},
		{"overlap", "database locked busy timeout retry", "When the SQLite database is locked, retry with a busy timeout instead of failing immediately."},
		{"overlap", "flaky network test rerun package smoke", "When the package smoke test fails on a transient fetch, rerun once after checking the error."},
		{"overlap", "goroutine leak context cancellation", "A missing context cancellation leaks goroutines; always cancel the context when the request ends."},
		{"overlap", "npm install peer dependency conflict", "Resolve npm peer dependency conflicts by aligning versions rather than forcing install."},
		{"overlap", "docker image layer cache invalidation", "Invalidate the Docker image layer cache when the base image tag changes."},
		{"overlap", "postgres index bloat vacuum analyze", "Run vacuum analyze when the Postgres index shows bloat after bulk deletes."},
		{"exact", "retry with a busy timeout", "When the SQLite database is locked, retry with a busy timeout instead of failing immediately."},
		{"exact", "invalidate the CI cache after a lockfile change", "The CI cache must be invalidated after a dependency lockfile change, otherwise stale artifacts break the build."},
		{"exact", "cancel the context when the request ends", "A missing context cancellation leaks goroutines; always cancel the context when the request ends."},
		{"synonym", "login credential lifetime handling", "Authentication token expiry causes 401 responses; refresh the token before issuing the request."},
		{"synonym", "continuous integration artifact freshness", "The CI cache must be invalidated after a dependency lockfile change, otherwise stale artifacts break the build."},
		{"synonym", "thread leakage on request completion", "A missing context cancellation leaks goroutines; always cancel the context when the request ends."},
		{"misleading", "package smoke test timeout retry", "When the package smoke test fails on a transient fetch, rerun once after checking the error."},
		{"misleading", "context timeout for database queries", "When the SQLite database is locked, retry with a busy timeout instead of failing immediately."},
		{"misleading", "authentication cache invalidation", "The CI cache must be invalidated after a dependency lockfile change, otherwise stale artifacts break the build."},
	}
}

// misleadingDistractors adds memories that share terms with a misleading query
// but are not the gold answer, to test robustness to lexical false positives.
func misleadingDistractors() []asymptoteobserve.LearningMemoryV1 {
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}
	return []asymptoteobserve.LearningMemoryV1{
		{ID: "distractor-smoke-timeout", Kind: asymptoteobserve.LearningMemoryKindConvention,
			Title: "smoke timeout", Body: "The package smoke test has a timeout retry policy for the network.", Project: project},
		{ID: "distractor-context-db", Kind: asymptoteobserve.LearningMemoryKindConvention,
			Title: "context db", Body: "Set a context timeout for database queries to avoid hanging.", Project: project},
		{ID: "distractor-auth-cache", Kind: asymptoteobserve.LearningMemoryKindConvention,
			Title: "auth cache", Body: "Authentication cache invalidation happens on logout.", Project: project},
	}
}

func buildSemanticStore(t *testing.T) (*Store, []semCase) {
	return buildSemanticStoreWith(t, NewHashedEmbedder())
}

func buildSemanticStoreWith(t *testing.T, e Embedder) (*Store, []semCase) {
	t.Helper()
	store := OpenSemantic(filepath.Join(t.TempDir(), "memory.db"), e)
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}
	cases := semanticCases()
	// Gold memories, one per case, with evidence so trust labels them verified.
	for i, c := range cases {
		m := asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("gold-%02d", i), CandidateID: fmt.Sprintf("cand-%02d", i),
			Kind:  asymptoteobserve.LearningMemoryKindDebuggingPattern,
			Title: fmt.Sprintf("lesson %d", i), Body: c.gold, Project: project,
			Evidence: []asymptoteobserve.LearningEvidenceV1{{TraceID: fmt.Sprintf("trace-%d", i), EventIDs: []string{"e1"}}},
		}
		if err := store.PutMemory(m); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range misleadingDistractors() {
		d.CandidateID = "cand-" + d.ID
		d.Evidence = []asymptoteobserve.LearningEvidenceV1{{TraceID: "trace-distractor"}}
		if err := store.PutMemory(d); err != nil {
			t.Fatal(err)
		}
	}
	return store, cases
}

func goldID(cases []semCase, i int) string { return fmt.Sprintf("gold-%02d", i) }

type semMetrics struct {
	recall1, recall5, mrr, ndcg5 float64
	n                            int
}

func evaluateRanking(ids []string, gold string) (float64, float64, float64, float64) {
	r1, r5, rr, ndcg := 0.0, 0.0, 0.0, 0.0
	for i, id := range ids {
		if id != gold {
			continue
		}
		rank := i + 1
		if rank == 1 {
			r1 = 1
		}
		if rank <= 5 {
			r5 = 1
			ndcg = 1 / math.Log2(float64(rank)+1)
		}
		rr = 1 / float64(rank)
		break
	}
	return r1, r5, rr, ndcg
}

func TestSemanticRetrievalQuality(t *testing.T) {
	store, cases := buildSemanticStore(t)
	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	type method struct {
		name string
		run  func(q string) []string
	}
	methods := []method{
		{"lexical", func(q string) []string {
			got, err := store.ListMemories(Query{ProjectID: "proj-a", Q: q, Limit: 5})
			if err != nil {
				t.Fatal(err)
			}
			return ids(got)
		}},
		{"semantic", func(q string) []string {
			hits, err := store.semanticSearchExact(db, Query{ProjectID: "proj-a"}, store.embedder.Embed(q), 5)
			if err != nil {
				t.Fatal(err)
			}
			out := make([]string, len(hits))
			for i, h := range hits {
				out[i] = h.memory.ID
			}
			return out
		}},
		{"hybrid", func(q string) []string {
			got, err := store.SearchHybrid(Query{ProjectID: "proj-a", Q: q}, HybridOptions{Limit: 5, CandidateK: 50})
			if err != nil {
				t.Fatal(err)
			}
			out := make([]string, len(got))
			for i, r := range got {
				out[i] = r.Memory.ID
			}
			return out
		}},
	}

	categories := []string{"overlap", "exact", "synonym", "misleading"}
	for _, m := range methods {
		overall := semMetrics{}
		perCat := map[string]semMetrics{}
		for i, c := range cases {
			ids := m.run(c.query)
			r1, r5, rr, ndcg := evaluateRanking(ids, goldID(cases, i))
			overall.n++
			overall.recall1 += r1
			overall.recall5 += r5
			overall.mrr += rr
			overall.ndcg5 += ndcg
			cm := perCat[c.category]
			cm.n++
			cm.recall1 += r1
			cm.recall5 += r5
			cm.mrr += rr
			cm.ndcg5 += ndcg
			perCat[c.category] = cm
		}
		fmt.Printf("SEMANTIC method=%s overall n=%d recall@1=%.3f recall@5=%.3f mrr=%.3f ndcg@5=%.3f\n",
			m.name, overall.n, overall.recall1/float64(overall.n), overall.recall5/float64(overall.n),
			overall.mrr/float64(overall.n), overall.ndcg5/float64(overall.n))
		for _, cat := range categories {
			cm := perCat[cat]
			if cm.n == 0 {
				continue
			}
			fmt.Printf("SEMANTIC method=%s category=%s n=%d recall@1=%.3f recall@5=%.3f mrr=%.3f ndcg@5=%.3f\n",
				m.name, cat, cm.n, cm.recall1/float64(cm.n), cm.recall5/float64(cm.n),
				cm.mrr/float64(cm.n), cm.ndcg5/float64(cm.n))
		}
	}
}

// TestHybridNoModelFallback confirms that without an embedder the hybrid search
// returns exactly the lexical ranking, so the default path is unchanged.
func TestHybridNoModelFallback(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}
	for i := 0; i < 20; i++ {
		if err := store.PutMemory(asymptoteobserve.LearningMemoryV1{
			ID: fmt.Sprintf("m-%02d", i), CandidateID: fmt.Sprintf("c-%02d", i),
			Kind:  asymptoteobserve.LearningMemoryKindConvention,
			Title: "note", Body: fmt.Sprintf("the needle lesson %d", i), Project: project,
		}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.SearchHybrid(Query{ProjectID: "proj-a", Q: "needle"}, HybridOptions{Limit: 5, CandidateK: 20})
	if err != nil {
		t.Fatal(err)
	}
	want, err := store.ListMemories(Query{ProjectID: "proj-a", Q: "needle", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("hybrid len %d, lexical len %d", len(got), len(want))
	}
	for i := range got {
		if got[i].Memory.ID != want[i].ID {
			t.Fatalf("hybrid[%d]=%s, lexical[%d]=%s", i, got[i].Memory.ID, i, want[i].ID)
		}
		if got[i].SemanticRank != 0 {
			t.Fatalf("semantic rank set without a model: %+v", got[i])
		}
	}
}

func TestSemanticIncrementalUpdate(t *testing.T) {
	store := OpenSemantic(filepath.Join(t.TempDir(), "memory.db"), NewHashedEmbedder())
	project := asymptoteobserve.LearningProjectV1{ID: "proj-a"}
	m := asymptoteobserve.LearningMemoryV1{
		ID: "m1", CandidateID: "c1", Kind: asymptoteobserve.LearningMemoryKindConvention,
		Title: "t", Body: "first body about tokens", Project: project,
	}
	if err := store.PutMemory(m); err != nil {
		t.Fatal(err)
	}
	db, err := store.db()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hash1 := ""
	if err := db.QueryRow(`SELECT content_hash FROM memory_embeddings WHERE memory_id = 'm1'`).Scan(&hash1); err != nil {
		t.Fatal(err)
	}
	// Re-writing identical content must not change the embedding hash.
	if err := store.PutMemory(m); err != nil {
		t.Fatal(err)
	}
	hash2 := ""
	if err := db.QueryRow(`SELECT content_hash FROM memory_embeddings WHERE memory_id = 'm1'`).Scan(&hash2); err != nil {
		t.Fatal(err)
	}
	if hash1 != hash2 {
		t.Fatalf("unchanged content re-embedded: %s -> %s", hash1, hash2)
	}
	// Changing the body must refresh the embedding.
	m.Body = "second body about refresh tokens"
	if err := store.PutMemory(m); err != nil {
		t.Fatal(err)
	}
	hash3 := ""
	if err := db.QueryRow(`SELECT content_hash FROM memory_embeddings WHERE memory_id = 'm1'`).Scan(&hash3); err != nil {
		t.Fatal(err)
	}
	if hash3 == hash2 {
		t.Fatalf("changed content not re-embedded")
	}
}

func BenchmarkSemanticExact(b *testing.B) {
	for _, n := range []int{1000, 10000, 100000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			store := OpenSemantic(filepath.Join(b.TempDir(), "memory.db"), NewHashedEmbedder())
			seedMemories(b, store, diverseDataset(n))
			if err := store.RebuildEmbeddings(); err != nil {
				b.Fatal(err)
			}
			db, err := store.db()
			if err != nil {
				b.Fatal(err)
			}
			defer db.Close()
			qvec := store.embedder.Embed("unique-needle recovery step")
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := store.semanticSearchExact(db, Query{ProjectID: "proj-a"}, qvec, 5); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
